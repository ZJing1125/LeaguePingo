//go:build windows

// perfprobe samples only a named process tree via Windows counters. It neither
// injects into games nor reads command lines, tokens, payloads or game memory.
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type counters struct {
	created uint64
	cpu     float64
	private uint64
	working uint64
}
type memoryCounters struct {
	Size                                                                                            uint32
	PageFaults                                                                                      uint32
	PeakWorking, Working, PeakPaged, Paged, PeakNonPaged, NonPaged, Pagefile, PeakPagefile, Private uintptr
}

var getMemory = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

func filetime(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
func readCounters(pid uint32) (counters, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return counters{}, err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return counters{}, err
	}
	mem := memoryCounters{}
	mem.Size = uint32(unsafe.Sizeof(mem))
	ok, _, err := getMemory.Call(uintptr(h), uintptr(unsafe.Pointer(&mem)), uintptr(mem.Size))
	if ok == 0 {
		return counters{}, err
	}
	return counters{created: filetime(created), cpu: float64(filetime(kernel)+filetime(user)) / 1e7, private: uint64(mem.Private), working: uint64(mem.Working)}, nil
}
func descendants(root uint32) (map[uint32]bool, error) {
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
func run() error {
	rootFlag := flag.Uint("pid", 0, "existing assistant PID; read-only sampling")
	executable := flag.String("exe", "", "launch an isolated hidden offline instance instead")
	output := flag.String("out", "", "CSV directory (required)")
	seconds := flag.Int("seconds", 22, "samples, 5 to 600")
	scenario := flag.String("scenario", "manual-labelled", "scenario label; never claims game A/B")
	openOverlay := flag.Bool("open-overlay", false, "isolated probe also creates and hides the overlay")
	flag.Parse()
	if *output == "" || *seconds < 5 || *seconds > 600 || (*rootFlag == 0) == (*executable == "") {
		return errors.New("supply -out, valid -seconds, and exactly one of -pid or -exe")
	}
	if *executable != "" && *seconds > 25 {
		return errors.New("isolated offline probe self-exits; use at most 25 samples")
	}
	dir, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	root := uint32(*rootFlag)
	var done chan error
	if *executable != "" {
		exe, err := filepath.Abs(*executable)
		if err != nil {
			return err
		}
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "LEAGUEPINGO_OFFLINE_SMOKE=1", "LEAGUEPINGO_SMOKE_DIR="+filepath.Join(dir, "isolated-settings"))
		if *openOverlay {
			cmd.Env = append(cmd.Env, "LEAGUEPINGO_SMOKE_OPEN_OVERLAY=1")
		} else {
			cmd.Env = append(cmd.Env, "LEAGUEPINGO_SMOKE_OPEN_OVERLAY=0")
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err = cmd.Start(); err != nil {
			return err
		}
		root = uint32(cmd.Process.Pid)
		done = make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		*scenario = "offline-hidden-main-overlay-unused-no-lcu"
		if *openOverlay {
			*scenario = "offline-hidden-overlay-created-no-lcu"
		}
		time.Sleep(4 * time.Second)
	}
	identity, err := readCounters(root)
	if err != nil {
		return errors.New("cannot read root process counters")
	}
	filename := filepath.Join(dir, "process-tree-"+time.Now().Format("20060102-150405")+".csv")
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err = writer.Write([]string{"scenario", "elapsedSeconds", "processCount", "cpuPercentMachine", "cpuPercentOneCore", "privateMiB", "workingSetSumMiB"}); err != nil {
		return err
	}
	previous := map[string]float64{}
	start := time.Now()
	last := start
	var sumCPU, peakPrivate, peakWS float64
	var samples int
	for i := 0; i <= *seconds; i++ {
		if i > 0 {
			time.Sleep(time.Second)
		}
		current, err := readCounters(root)
		if err != nil || current.created != identity.created {
			break
		}
		members, err := descendants(root)
		if err != nil {
			return err
		}
		var delta float64
		var private, ws uint64
		count := 0
		for id := range members {
			c, err := readCounters(id)
			if err != nil || c.created < identity.created {
				continue
			}
			key := fmt.Sprintf("%d/%d", id, c.created)
			if old, ok := previous[key]; ok {
				delta += max(0, c.cpu-old)
			}
			previous[key] = c.cpu
			private += c.private
			ws += c.working
			count++
		}
		now := time.Now()
		elapsed := now.Sub(last).Seconds()
		last = now
		if i == 0 {
			continue
		}
		oneCore := 100 * delta / elapsed
		machine := oneCore / float64(runtime.NumCPU())
		priv := float64(private) / (1 << 20)
		working := float64(ws) / (1 << 20)
		sumCPU += machine
		peakPrivate = max(peakPrivate, priv)
		peakWS = max(peakWS, working)
		samples++
		f := func(n float64) string { return strconv.FormatFloat(n, 'f', 3, 64) }
		if err = writer.Write([]string{*scenario, f(now.Sub(start).Seconds()), strconv.Itoa(count), f(machine), f(oneCore), f(priv), f(working)}); err != nil {
			return err
		}
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		return err
	}
	if samples < 2 {
		return errors.New("insufficient samples")
	}
	if done != nil {
		select {
		case err := <-done:
			if err != nil {
				return err
			}
		case <-time.After(20 * time.Second):
			return errors.New("isolated probe did not self-exit; no process was stopped")
		}
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Scenario          string  `json:"scenario"`
		Samples           int     `json:"samples"`
		LogicalProcessors int     `json:"logicalProcessors"`
		AverageCPU        float64 `json:"averageCPUPercentMachine"`
		PeakPrivate       float64 `json:"peakPrivateMiB"`
		PeakWorking       float64 `json:"peakWorkingSetSumMiB"`
		Limitations       string  `json:"limitations"`
	}{*scenario, samples, runtime.NumCPU(), sumCPU / float64(samples), peakPrivate, peakWS, "Includes readable descendant WebView2 counters. Working sets may double-count shared pages; one-second samples miss short-lived processes. No FPS, ping or game A/B measured."})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
