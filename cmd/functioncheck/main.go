// functioncheck audits tracked production Go, including target-specific files.
// Run from a checkout; no game client, network or extra dependency is required.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"leaguepingo/internal/functioncheck"
)

func run() error {
	rootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return errors.New("functioncheck requires a Git checkout")
	}
	root := strings.TrimSpace(string(rootBytes))
	tracked, err := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go").Output()
	if err != nil {
		return err
	}
	count := 0
	for _, name := range strings.Split(string(tracked), "\x00") {
		if name == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return err
		}
		findings, err := functioncheck.CheckFile(name, data)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			fmt.Fprintf(os.Stderr, "%s:%d: %s has %d lines (limit %d)\n", name, finding.Line, finding.Name, finding.Lines, functioncheck.MaxLines)
			count++
		}
	}
	if count > 0 {
		return fmt.Errorf("%d production functions/closures exceed the line limit", count)
	}
	fmt.Println("PASS: managed production Go functions and closures are at most 100 physical lines; tests excluded.")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
