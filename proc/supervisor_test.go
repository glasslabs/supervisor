package proc_test

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glasslabs/supervisor/internal/exec/exectest"
	"github.com/glasslabs/supervisor/proc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	flag.Parse()

	if os.Getenv("EXEC_TEST_PID") == "" {
		_ = os.Setenv("EXEC_TEST_PID", strconv.Itoa(os.Getpid()))
		os.Exit(m.Run())
	}

	os.Exit(exectest.Run(flag.Args(), exectest.DefaultCommands))
}

func TestSupervisor_Info(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "wait", time.Minute.String())
	sup := proc.New(exe)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	require.Eventually(t, func() bool {
		return sup.Info().PID > 0
	}, 5*time.Second, 10*time.Millisecond)

	info := sup.Info()
	assert.Greater(t, info.PID, 0)
	assert.False(t, info.Started.IsZero())
	assert.Equal(t, int32(0), info.Restarts)
}

func TestSupervisor_Lines(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "echo", "hello world")
	sup := proc.New(exe)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	require.Eventually(t, func() bool {
		lines := sup.Lines()
		if len(lines) == 0 {
			return false
		}
		return lines[0] == "hello world"
	}, 5*time.Second, 10*time.Millisecond)
}

func TestSupervisor_Follow(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "echo", "streamed line")
	sup := proc.New(exe)

	require.NoError(t, sup.Start(t.Context()))

	ch := sup.Follow(t.Context())

	var got []string
	require.Eventually(t, func() bool {
		for {
			select {
			case line, ok := <-ch:
				if !ok {
					return false
				}
				got = append(got, line)
				if len(got) >= 1 {
					return true
				}
			default:
				return false
			}
		}
	}, 5*time.Second, 10*time.Millisecond)

	assert.Contains(t, got, "streamed line")
}

func TestSupervisor_Restart(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "wait", time.Minute.String())
	sup := proc.New(exe)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	require.Eventually(t, func() bool {
		return sup.Info().PID > 0
	}, 5*time.Second, 10*time.Millisecond)

	firstPID := sup.Info().PID

	sup.Restart()

	require.Eventually(t, func() bool {
		info := sup.Info()
		return info.PID > 0 && info.PID != firstPID
	}, 10*time.Second, 10*time.Millisecond)
}

func TestSupervisor_RestartIncrementsCount(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "exit", "0")
	sup := proc.New(exe)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	require.Eventually(t, func() bool {
		return sup.Info().Restarts > 0
	}, 10*time.Second, 10*time.Millisecond)
}

func TestSupervisor_ConditionHoldsProcess(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "wait", time.Minute.String())
	cond := func() (bool, string) { return false, "not ready" }
	sup := proc.New(exe, proc.WithCondition(cond))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	// Give the supervisor time to attempt a start; the process must not start.
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, sup.Info().PID)
}

func TestSupervisor_ConditionWritesReasonToBuffer(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "wait", time.Minute.String())
	cond := func() (bool, string) { return false, "missing config" }
	sup := proc.New(exe, proc.WithCondition(cond))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	require.Eventually(t, func() bool {
		for _, line := range sup.Lines() {
			if strings.Contains(line, "missing config") {
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond)
}

func TestSupervisor_StartsWhenConditionPasses(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "wait", time.Minute.String())

	ready := make(chan struct{})
	cond := func() (bool, string) {
		select {
		case <-ready:
			return true, ""
		default:
			return false, "not ready"
		}
	}
	sup := proc.New(exe, proc.WithCondition(cond))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	require.NoError(t, sup.Start(ctx))

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, sup.Info().PID, "process should not have started yet")

	close(ready)

	require.Eventually(t, func() bool {
		return sup.Info().PID > 0
	}, 30*time.Second, 10*time.Millisecond)
}

func TestSupervisor_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	exe := exectest.New(t, "wait", time.Minute.String())
	sup := proc.New(exe)

	ctx, cancel := context.WithCancel(t.Context())

	require.NoError(t, sup.Start(ctx))

	require.Eventually(t, func() bool {
		return sup.Info().PID > 0
	}, 5*time.Second, 10*time.Millisecond, "Process never started")

	cancel()

	require.Eventually(t, func() bool {
		return sup.Info().PID == 0
	}, 10*time.Second, 10*time.Millisecond)
}
