// Package exectest provides helpers for testing code that spawns subprocesses
// by re-entering the test binary as the child process.
package exectest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/glasslabs/supervisor/internal/exec"
	"github.com/stretchr/testify/assert"
)

var (
	// Path returns the path to the test executable.
	Path = sync.OnceValues(os.Executable)

	// DefaultCommands contains built-in helper commands available to re-entered test binaries.
	DefaultCommands = map[string]func([]string) int{
		"exit": cmdExitCode,
		"echo": cmdEcho,
		"wait": cmdWait,
	}
)

// Run dispatches to the named command in commands using args.
// It is intended to be called from TestMain when the process has been re-entered.
func Run(args []string, commandFns map[string]func([]string) int) int {
	code, err := run(args, commandFns)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error running test command: %s\n", err)
		return 2
	}
	return code
}

func run(args []string, commandFns map[string]func([]string) int) (int, error) {
	if len(args) == 0 {
		return 0, errors.New("no test command")
	}

	name, rest := args[0], args[1:]
	f, ok := commandFns[name]
	if !ok {
		return 0, errors.New("unknown test command: " + name)
	}
	return f(rest), nil
}

// New returns a new test command.
func New(t *testing.T, name string, args ...string) *exec.Executable {
	t.Helper()

	path, args := testCommand(t, name, args...)

	return &exec.Executable{
		Path: path,
		Args: args,
		Envs: os.Environ(),
	}
}

func testCommand(t *testing.T, name string, args ...string) (string, []string) {
	t.Helper()

	exe, err := Path()
	if err != nil {
		assert.Failf(t, "Could not fund executable: %s", err.Error())
		return "", nil
	}

	return exe, append([]string{name}, args...)
}

func cmdExitCode(args []string) int {
	if len(args) == 0 {
		return 0
	}
	code, err := strconv.Atoi(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "exit: invalid code %q\n", args[0])
		return 2
	}
	return code
}

func cmdEcho(args []string) int {
	_, _ = fmt.Fprintln(os.Stdout, strings.Join(args, " "))
	return 0
}

func cmdWait(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()

	var dur time.Duration
	if len(args) > 0 {
		dur, _ = time.ParseDuration(args[0])
	}
	if dur == 0 {
		dur = time.Hour // Forever.
	}

	select {
	case <-ctx.Done():
	case <-time.After(dur):
	}
	return 0
}
