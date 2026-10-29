//go:build !windows

package diagnostics

import (
	"errors"
	"runtime"
)

func resourceSupported() bool                 { return false }
func logicalProcessors() int                  { return runtime.NumCPU() }
func readResources() (resourceReading, error) { return resourceReading{}, errors.ErrUnsupported }
