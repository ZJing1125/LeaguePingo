//go:build windows

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// A hidden, isolated Wails process: no League engine, tray, game API or global
// shortcuts. This checks the native single-instance IPC and window operations.
func TestDesktopProbeProcess(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_DESKTOP_PROBE") != "1" {
		t.Skip("subprocess helper")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// testing runs helpers on a different thread from package init/main.
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 {
			t.Fatal(err)
		}
	}
	defer ole.CoUninitialize()
	dir := os.Getenv("LEAGUEPINGO_PROBE_DIR")
	var app *application.App
	options := singleInstanceOptions(func() {
		_ = os.WriteFile(filepath.Join(dir, "activated"), []byte("ok"), 0600)
		app.Quit()
	})
	options.UniqueID = os.Getenv("LEAGUEPINGO_PROBE_ID")
	app = application.New(application.Options{Name: "LeaguePingo isolated test", SingleInstance: options})
	windowOptions := mainWindowOptions(true)
	windowOptions.Name = "probe"
	windowOptions.URL = ""
	windowOptions.HTML = "<html><body>Isolated desktop smoke test</body></html>"
	w := app.Window.NewWithOptions(windowOptions)
	var probeErr error
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if w.IsIgnoreMouseEvents() {
			probeErr = fmt.Errorf("native window must accept mouse input")
		}
		w.SetAlwaysOnTop(true)
		w.SetAlwaysOnTop(false)
		w.SetPosition(120, 140)
		if x, y := w.Position(); x != 120 || y != 140 {
			probeErr = fmt.Errorf("native position did not change")
		}
		if probeErr != nil {
			_ = os.WriteFile(filepath.Join(dir, "failed"), []byte(probeErr.Error()), 0600)
			app.Quit()
			return
		}
		_ = os.WriteFile(filepath.Join(dir, "ready"), []byte("ok"), 0600)
	})
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeDesktopSingleInstance(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_DESKTOP_TEST") != "1" {
		t.Skip("set LEAGUEPINGO_DESKTOP_TEST=1 for hidden native desktop checks")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	newProbe := func() *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDesktopProbeProcess$", "-test.v")
		cmd.Env = append(os.Environ(), "LEAGUEPINGO_DESKTOP_PROBE=1", "LEAGUEPINGO_PROBE_DIR="+dir, fmt.Sprintf("LEAGUEPINGO_PROBE_ID=com.leaguepingo.test.p%d", os.Getpid()))
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd
	}
	first := newProbe()
	var firstOutput bytes.Buffer
	first.Stdout, first.Stderr = &firstOutput, &firstOutput
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- first.Wait() }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}()
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "failed")); err == nil {
			t.Fatalf("native window check: %s", data)
		}
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("first instance exited before readiness: %v %s", err, firstOutput.String())
		case <-ctx.Done():
			t.Fatal("native startup timed out")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if output, err := newProbe().CombinedOutput(); err != nil {
		t.Fatalf("duplicate did not exit cleanly: %v %s", err, output)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first instance failed: %v", err)
		}
		done <- nil
	case <-ctx.Done():
		t.Fatal("first instance did not receive duplicate notification")
	}
	if _, err := os.Stat(filepath.Join(dir, "activated")); err != nil {
		t.Fatal("activation not delivered")
	}
	t.Log("native duplicate launch exited and notified first instance; hidden window position and mouse input checks passed")
}
