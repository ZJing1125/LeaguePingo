//go:build windows

package diagnostics

import (
	"os"
	"runtime"
	"sort"

	"leaguepingo/internal/processstats"
)

func resourceSupported() bool { return true }
func logicalProcessors() int  { return runtime.NumCPU() }
func readResources() (resourceReading, error) {
	root := uint32(os.Getpid())
	c, err := processstats.Read(root)
	if err != nil {
		return resourceReading{}, err
	}
	r := resourceReading{root: processIdentity{root, c.Created}, processes: map[processIdentity]processCounter{}}
	r.processes[r.root] = processCounter{c.CPUSeconds, c.PrivateBytes, c.WorkingSetBytes}
	ids, err := processstats.Descendants(root)
	if err != nil {
		return resourceReading{}, err
	}
	ordered := make([]uint32, 0, len(ids))
	for id := range ids {
		if id != root {
			ordered = append(ordered, id)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	if len(ordered) >= resourceProcessLimit {
		r.partial = true
		ordered = ordered[:resourceProcessLimit-1]
	}
	for _, id := range ordered {
		p, e := processstats.Read(id)
		if e != nil || p.Created < c.Created {
			r.partial = true
			continue
		}
		r.processes[processIdentity{id, p.Created}] = processCounter{p.CPUSeconds, p.PrivateBytes, p.WorkingSetBytes}
	}
	return r, nil
}
