package services

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// running tracks commands started by ExecuteCommand, so they can be stopped
// when the application quits.
var running = struct {
	sync.Mutex
	cmds map[*exec.Cmd]bool
}{cmds: map[*exec.Cmd]bool{}}

// RunningCommands returns the command lines of commands still running.
func RunningCommands() []string {
	running.Lock()
	defer running.Unlock()
	var lines []string
	for cmd := range running.cmds {
		lines = append(lines, strings.Join(cmd.Args, " "))
	}
	return lines
}

// StopRunningCommands sends SIGTERM to every running command's process group
// (commands are started with Setsid, so a group holds brew and anything it
// spawned), waits up to timeout for them to exit, then sends SIGKILL.
func StopRunningCommands(timeout time.Duration) {
	signalAll := func(sig syscall.Signal) {
		running.Lock()
		defer running.Unlock()
		for cmd := range running.cmds {
			if cmd.Process == nil {
				continue
			}
			// Negative pid: the whole process group (pgid == pid with Setsid).
			if syscall.Kill(-cmd.Process.Pid, sig) != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}

	signalAll(syscall.SIGTERM)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(RunningCommands()) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	signalAll(syscall.SIGKILL)
}

// ExecuteCommand runs a command, streaming stdout/stderr to the provided writer in real time.
func ExecuteCommand(cmd *exec.Cmd, output io.Writer) error {
	stdoutPipe, stdoutWriter := io.Pipe()
	stderrPipe, stderrWriter := io.Pipe()
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Start(); err != nil {
		return err
	}

	running.Lock()
	running.cmds[cmd] = true
	running.Unlock()

	var wg sync.WaitGroup
	wg.Add(3)

	cmdErrCh := make(chan error, 1)

	go func() {
		defer wg.Done()
		defer stdoutWriter.Close()
		defer stderrWriter.Close()
		err := cmd.Wait()
		running.Lock()
		delete(running.cmds, cmd)
		running.Unlock()
		cmdErrCh <- err
	}()

	streamPipe := func(pipe *io.PipeReader) {
		defer wg.Done()
		defer pipe.Close()
		buf := make([]byte, 1024)
		for {
			n, err := pipe.Read(buf)
			if n > 0 {
				_, _ = output.Write(buf[:n]) // #nosec G104
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(output, "\nError: %v\n", err)
				}
				break
			}
		}
	}

	go streamPipe(stdoutPipe)
	go streamPipe(stderrPipe)

	wg.Wait()

	return <-cmdErrCh
}
