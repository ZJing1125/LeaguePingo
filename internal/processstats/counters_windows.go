//go:build windows

// Package processstats reads OS counters, never process memory contents or arguments.
package processstats

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Counters struct {
	Created         uint64
	CPUSeconds      float64
	PrivateBytes    uint64
	WorkingSetBytes uint64
}
type memoryCounters struct {
	Size                                                                                            uint32
	PageFaults                                                                                      uint32
	PeakWorking, Working, PeakPaged, Paged, PeakNonPaged, NonPaged, Pagefile, PeakPagefile, Private uintptr
}

var getMemory = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

func filetime(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
func Read(pid uint32) (Counters, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return Counters{}, err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return Counters{}, err
	}
	mem := memoryCounters{}
	mem.Size = uint32(unsafe.Sizeof(mem))
	ok, _, err := getMemory.Call(uintptr(h), uintptr(unsafe.Pointer(&mem)), uintptr(mem.Size))
	if ok == 0 {
		return Counters{}, err
	}
	return Counters{Created: filetime(created), CPUSeconds: float64(filetime(kernel)+filetime(user)) / 1e7, PrivateBytes: uint64(mem.Private), WorkingSetBytes: uint64(mem.Working)}, nil
}
func Descendants(root uint32) (map[uint32]bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	parents := map[uint32]uint32{}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		parents[entry.ProcessID] = entry.ParentProcessID
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	members := map[uint32]bool{root: true}
	for changed := true; changed; {
		changed = false
		for id, parent := range parents {
			if members[parent] && !members[id] {
				members[id] = true
				changed = true
			}
		}
	}
	return members, nil
}
