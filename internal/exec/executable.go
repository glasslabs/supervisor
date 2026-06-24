package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

// Executable describe an executable and its environment.
type Executable struct {
	Path        string
	Args        []string
	Envs        []string
	SysProcAttr *syscall.SysProcAttr

	pid atomic.Int64
}

// Run starts the executable asynchronously.
func (exe *Executable) Run(ctx context.Context, stdOut, stdErr io.Writer) (<-chan error, error) {
	//nolint:gosec // Paths are from trusted config.
	cmd := exec.CommandContext(ctx, filepath.Clean(exe.Path), exe.Args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Env = exe.Envs
	cmd.SysProcAttr = exe.SysProcAttr

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("starting executable: %w", err)
	}

	exe.pid.Store(int64(cmd.Process.Pid))

	shutdownCh := make(chan error)
	go func() {
		defer close(shutdownCh)
		defer exe.pid.Store(0)

		err = cmd.Wait()
		if err != nil {
			if isNormalExit(err) {
				err = nil
			}

			shutdownCh <- err
		}
	}()
	return shutdownCh, nil
}

// isNormalExit detects if the process has exited normally.
// It is expected that the container will be SIGTERMed, so we consider this a normal exit.
func isNormalExit(err error) bool {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		status, _ := exitErr.Sys().(syscall.WaitStatus)
		if (exitErr.ExitCode() == 0 && status.Signal() == -1) ||
			(exitErr.ExitCode() == -1 && status.Signal() == syscall.SIGTERM) {
			return true
		}
	}
	return false
}

// PID returns the pid of the running process or 0.
func (exe *Executable) PID() int {
	return int(exe.pid.Load())
}
