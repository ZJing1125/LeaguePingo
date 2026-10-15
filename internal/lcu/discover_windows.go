//go:build windows

package lcu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func Discover() (*Client, error) {
	var paths []string
	if override := os.Getenv("LEAGUEPINGO_LOCKFILE"); override != "" {
		paths = append(paths, override)
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return DiscoverFromPaths(paths)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if !strings.EqualFold(name, "LeagueClient.exe") {
			continue
		}
		handle, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if openErr != nil {
			continue
		}
		buffer := make([]uint16, windows.MAX_PATH*4)
		size := uint32(len(buffer))
		queryErr := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size)
		windows.CloseHandle(handle)
		if queryErr == nil {
			paths = append(paths, filepath.Join(filepath.Dir(windows.UTF16ToString(buffer[:size])), "lockfile"))
		}
	}
	client, err := DiscoverFromPaths(paths)
	if err != nil {
		return nil, fmt.Errorf("%w：请启动国服客户端，或设置 LEAGUEPINGO_LOCKFILE", ErrNotFound)
	}
	return client, nil
}
