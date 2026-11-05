//go:build windows

package main

import (
	"bytes"
	"encoding/csv"
	"math"
	"os"
	"strconv"
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

// Sample only this test process, without launching the assistant or a client.
// This validates CSV/counter handling, not game performance or an FPS baseline.
func TestSampleOwnProcessWritesFiniteNonnegativeCSV(t *testing.T) {
	root := uint32(os.Getpid())
	identity, err := readCounters(root)
	if err != nil {
		t.Fatal("cannot read own test process counters")
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	metrics, err := sampleProcessTree(root, identity, 2, "synthetic-self", writer)
	writer.Flush()
	if err != nil || writer.Error() != nil || metrics.samples != 2 || metrics.sumCPU < 0 || metrics.peakPrivate <= 0 || metrics.peakWS <= 0 {
		t.Fatal("own-process counter sample or CSV failed")
	}
	rows, err := csv.NewReader(&output).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatal("sample row count changed")
	}
	for _, row := range rows {
		if len(row) != 7 || row[0] != "synthetic-self" {
			t.Fatal("CSV schema or scenario changed")
		}
		for _, cell := range row[1:] {
			if number, err := strconv.ParseFloat(cell, 64); err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
				t.Fatal("counter cell was missing or negative")
			}
		}
	}
}

func TestSampleRejectsReusedRootIdentity(t *testing.T) {
	root := uint32(os.Getpid())
	identity, err := readCounters(root)
	if err != nil {
		t.Fatal("cannot read own test process counters")
	}
	identity.created++
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	metrics, err := sampleProcessTree(root, identity, 2, "synthetic-stale", writer)
	writer.Flush()
	if err != nil || metrics.samples != 0 || output.Len() != 0 {
		t.Fatal("reused root identity was sampled")
	}
}
