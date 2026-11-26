package services

import (
	"io"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestStopRunningCommands(t *testing.T) {
	// A shell with a child, in its own session like brew commands.
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	done := make(chan error, 1)
	go func() { done <- ExecuteCommand(cmd, io.Discard) }()

	deadline := time.Now().Add(2 * time.Second)
	for len(RunningCommands()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("command never registered as running")
		}
		time.Sleep(10 * time.Millisecond)
	}

	start := time.Now()
	StopRunningCommands(2 * time.Second)

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected the stopped command to report an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("command still running after StopRunningCommands")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("stopping took %v; SIGTERM should have been enough", elapsed)
	}
	if n := len(RunningCommands()); n != 0 {
		t.Errorf("%d commands still registered", n)
	}
}
