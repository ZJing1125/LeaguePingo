//go:build windows

package main

import (
	"os"
	"testing"
	"time"
)

var workload uint64

func TestNativeCounterCPUAndProcessTree(t *testing.T) {
	pid := uint32(os.Getpid())
	before, err := readCounters(pid)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		workload++
	}
	after, err := readCounters(pid)
	if err != nil || after.created != before.created || after.cpu <= before.cpu || after.private == 0 || after.working == 0 {
		t.Fatal("native CPU/memory counters invalid", err)
	}
	members, err := descendants(pid)
	if err != nil || !members[pid] {
		t.Fatal("root missing from process tree", err)
	}
}
