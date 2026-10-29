//go:build windows

package lcu

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"leaguepingo/internal/apperrors"
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
		code := apperrors.ClientUnavailable
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			code = apperrors.ClientPermissionDenied
		}
		return nil, errors.Join(apperrors.New(code), ErrNotFound)
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
		failures = append(failures, apperrors.New(apperrors.ClientConfigurationInvalid))
	}
	if !foundProcess {
		failures = append(failures, apperrors.New(apperrors.ClientNotRunning))
	}
	if !foundProcess && len(failures) == 1 {
		return nil, ErrNotFound
	}
	// Keep only classifications from process/lockfile discovery, never raw OS text.
	failures = append(failures, ErrNotFound)
	return nil, errors.Join(failures...)
}

func discoverProcess(pid uint32) (*Client, error) {
	// No PROCESS_VM_READ, injection, elevation or game-process access is needed.
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, apperrors.New(apperrors.ClientPermissionDenied)
		}
		return nil, apperrors.New(apperrors.ClientUnavailable)
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
		lockErr = apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	args, err := processCommandLineArgs(handle)
	if err != nil {
		return nil, errors.Join(err, lockErr)
	}
	cred, err := ParseConnectionArgs(args)
	if err != nil {
		return nil, errors.Join(err, lockErr)
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
		return nil, apperrors.New(apperrors.ClientPermissionDenied)
	}
	headerSize := uint32(unsafe.Sizeof(windows.NTUnicodeString{}))
	if required < headerSize || required > 128<<10 {
		return nil, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	data := make([]byte, required)
	err = windows.NtQueryInformationProcess(handle, windows.ProcessCommandLineInformation, unsafe.Pointer(&data[0]), required, &required)
	if err != nil {
		return nil, apperrors.New(apperrors.ClientPermissionDenied)
	}
	value := (*windows.NTUnicodeString)(unsafe.Pointer(&data[0]))
	base := uintptr(unsafe.Pointer(&data[0]))
	start := uintptr(unsafe.Pointer(value.Buffer))
	length := uintptr(value.Length)
	if length == 0 || length%2 != 0 || start < base || start-base < uintptr(headerSize) || start-base > uintptr(len(data)) || length > uintptr(len(data))-(start-base) || (start-base)%2 != 0 {
		return nil, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	chars := unsafe.Slice((*uint16)(unsafe.Pointer(&data[start-base])), int(length/2))
	args, err := windows.DecomposeCommandLine(windows.UTF16ToString(chars))
	if err != nil {
		return nil, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	return args, nil
}
