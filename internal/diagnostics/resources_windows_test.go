//go:build windows

package diagnostics

import (
	"context"
	"testing"
)

func TestNativeResourceSampleUsesOwnReadableTree(t *testing.T) {
	r := NewResources()
	r.SetActive(true)
	r.sample(context.Background())
	m := r.Snapshot()
	if !m.Supported || m.MemorySamples != 1 || m.LastProcesses < 1 || m.LastPrivateBytes == nil || *m.LastPrivateBytes == 0 || m.LastWorkingSetBytes == nil || *m.LastWorkingSetBytes == 0 || m.LastCPUPercentMachine != nil {
		t.Fatal("invalid native counter baseline", m)
	}
	r.SetActive(false)
}
