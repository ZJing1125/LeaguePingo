//go:build windows

package lcu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func Discover() (*Client, error) {
	var failures []error
	if override := os.Getenv("LEAGUEPINGO_LOCKFILE"); override != "" {
		if client, err := DiscoverFromPaths([]string{override}); err == nil {
			return client, nil
		} else {
			failures = append(failures, err)
		}
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("%w：无法枚举客户端进程", ErrNotFound)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	foundProcess := false
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if !strings.EqualFold(name, "LeagueClient.exe") && !strings.EqualFold(name, "LeagueClientUx.exe") {
			continue
		}
		foundProcess = true
		if client, discoverErr := discoverProcess(entry.ProcessID); discoverErr == nil {
			return client, nil
		} else {
			failures = append(failures, discoverErr)
		}
	}
	if err != nil && !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		failures = append(failures, errors.New("客户端进程枚举未完成"))
	}
	if !foundProcess {
		failures = append(failures, errors.New("未发现 LeagueClient 或 LeagueClientUx 进程，请先启动客户端"))
	}
	// De-duplicate diagnostics from the two processes; never expose command lines.
	var messages []string
	seen := make(map[string]bool)
	for _, failure := range failures {
		message := failure.Error()
		if !seen[message] {
			seen[message] = true
			messages = append(messages, message)
		}
	}
	return nil, fmt.Errorf("%w：%s", ErrNotFound, strings.Join(messages, "；"))
}

func discoverProcess(pid uint32) (*Client, error) {
	// No PROCESS_VM_READ, injection, elevation or game-process access is needed.
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, errors.New("已发现客户端，但无权查询连接信息；请检查助手与客户端的运行权限")
		}
		return nil, errors.New("客户端进程已退出或暂时无法访问")
	}
	defer windows.CloseHandle(handle)
	var lockErr error
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err == nil {
		path := filepath.Join(filepath.Dir(windows.UTF16ToString(buffer[:size])), "lockfile")
		client, err := DiscoverFromPaths([]string{path})
		if err == nil {
			return client, nil
		}
		lockErr = err
	} else {
		lockErr = errors.New("无法获取客户端安装路径")
	}
	args, err := processCommandLineArgs(handle)
	if err != nil {
		return nil, errors.Join(lockErr, err)
	}
	cred, err := ParseConnectionArgs(args)
	if err != nil {
		return nil, errors.Join(lockErr, err)
	}
	return NewClient(cred), nil
}

// ProcessCommandLineInformation lets Windows return the client's startup
// metadata without reading its address space. Keep the buffer bounded and
// validate the returned UNICODE_STRING before accessing its contents.
func processCommandLineArgs(handle windows.Handle) ([]string, error) {
	var required uint32
	err := windows.NtQueryInformationProcess(handle, windows.ProcessCommandLineInformation, nil, 0, &required)
	if err != nil && !errors.Is(err, windows.STATUS_INFO_LENGTH_MISMATCH) && !errors.Is(err, windows.STATUS_BUFFER_TOO_SMALL) {
		return nil, errors.New("无法查询客户端启动参数，请检查运行权限或客户端是否已退出")
	}
	headerSize := uint32(unsafe.Sizeof(windows.NTUnicodeString{}))
	if required < headerSize || required > 128<<10 {
		return nil, errors.New("客户端启动参数长度无效")
	}
	data := make([]byte, required)
	err = windows.NtQueryInformationProcess(handle, windows.ProcessCommandLineInformation, unsafe.Pointer(&data[0]), required, &required)
	if err != nil {
		return nil, errors.New("无法读取客户端启动参数，请检查运行权限或稍后重试")
	}
	value := (*windows.NTUnicodeString)(unsafe.Pointer(&data[0]))
	base := uintptr(unsafe.Pointer(&data[0]))
	start := uintptr(unsafe.Pointer(value.Buffer))
	length := uintptr(value.Length)
	if length == 0 || length%2 != 0 || start < base || start-base < uintptr(headerSize) || start-base > uintptr(len(data)) || length > uintptr(len(data))-(start-base) || (start-base)%2 != 0 {
		return nil, errors.New("客户端启动参数结构无效")
	}
	chars := unsafe.Slice((*uint16)(unsafe.Pointer(&data[start-base])), int(length/2))
	args, err := windows.DecomposeCommandLine(windows.UTF16ToString(chars))
	if err != nil {
		return nil, errors.New("客户端启动参数无法解析")
	}
	return args, nil
}
