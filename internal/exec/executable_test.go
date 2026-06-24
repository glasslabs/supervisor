package exec_test

import (
	"flag"
	"io"
	"os"
	stdexec "os/exec"
	"strconv"
	"syscall"
	"testing"

	"github.com/glasslabs/supervisor/internal/exec"
	"github.com/glasslabs/supervisor/internal/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	flag.Parse()

	pid := os.Getpid()
	if os.Getenv("EXEC_TEST_PID") == "" {
		_ = os.Setenv("EXEC_TEST_PID", strconv.Itoa(pid))

		code := m.Run()
		os.Exit(code)
	}

	code := exectest.Run(flag.Args(), exectest.DefaultCommands)
	os.Exit(code)
}

func TestExecutable_Run(t *testing.T) {
	tests := []struct {
		name         string
		exe          *exec.Executable
		wantExitCode int
		wantSignal   syscall.Signal
		wantErr      bool
	}{
		{
			name: "handles normal exit",
			exe:  exectest.New(t, "exit", "0"),
		},
		{
			name:         "handles non-zero exit",
			exe:          exectest.New(t, "exit", "1"),
			wantExitCode: 1,
			wantSignal:   -1,
			wantErr:      true,
		},
		{
			name:       "returns error for invalid executable",
			exe:        &exec.Executable{Path: "not_echo"},
			wantSignal: 1,
			wantErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			errCh, err := test.exe.Run(t.Context(), io.Discard, io.Discard)

			if test.wantErr && err != nil {
				require.Error(t, err)
				return
			}

			select {
			case <-t.Context().Done():
				t.FailNow()
			case err = <-errCh:
			}

			if test.wantErr {
				var exitErr *stdexec.ExitError
				require.ErrorAs(t, err, &exitErr)

				assert.Equal(t, test.wantExitCode, exitErr.ExitCode())
				assert.Equal(t, test.wantSignal, exitErr.Sys().(syscall.WaitStatus).Signal())
				return
			}

			assert.NoError(t, err)
		})
	}
}
