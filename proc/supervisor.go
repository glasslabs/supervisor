// Package proc manages the supervised glass child process.
package proc

import (
	"container/ring"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glasslabs/supervisor/internal/exec"
)

const (
	ringSize        = 2000
	condInitBackoff = time.Second
	condMaxBackoff  = 60 * time.Second
)

// Condition reports whether the supervised process should be started.
// If it returns false, reason is written to the output buffer.
type Condition func() (ok bool, reason string)

// Option configures a Supervisor.
type Option func(*Supervisor)

// WithCondition sets a condition that must pass before the process is started.
// The supervisor checks the condition before each start attempt, backing off
// exponentially (up to condMaxBackoff) between failed checks.
func WithCondition(cond Condition) Option {
	return func(s *Supervisor) {
		s.cond = cond
	}
}

// Info holds a snapshot of supervisor state.
type Info struct {
	PID      int
	Restarts int32
	Started  time.Time
}

// Supervisor starts and supervises a process, capturing its output.
type Supervisor struct {
	exe  *exec.Executable
	cond Condition

	mu       sync.Mutex
	cancelFn func()
	started  time.Time
	restarts atomic.Int32

	ring *lineRingBuffer
}

// New returns a new Supervisor.
func New(exe *exec.Executable, opts ...Option) *Supervisor {
	s := &Supervisor{
		exe:  exe,
		ring: newRingBuffer(ringSize),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start begins the supervision loop in a background goroutine.
func (s *Supervisor) Start(ctx context.Context) error {
	go s.loop(ctx)
	return nil
}

// Restart sends SIGTERM to the running process. The supervision loop restarts it.
func (s *Supervisor) Restart() {
	s.mu.Lock()
	cancel := s.cancelFn
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Info returns a snapshot of current supervisor state.
func (s *Supervisor) Info() Info {
	s.mu.Lock()
	pid := s.exe.PID()
	started := s.started
	s.mu.Unlock()

	return Info{
		PID:      pid,
		Restarts: s.restarts.Load(),
		Started:  started,
	}
}

// Lines returns the current ring buffer contents in chronological order.
func (s *Supervisor) Lines() []string {
	return s.ring.Lines()
}

// Follow returns a channel that receives new log lines until ctx is cancelled.
func (s *Supervisor) Follow(ctx context.Context) <-chan string {
	return s.ring.Stream(ctx)
}

func (s *Supervisor) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !s.waitForCondition(ctx) {
			return
		}

		if err := s.run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			_, _ = s.ring.Write([]byte("Process exited with error: " + err.Error() + "\n"))
		}

		s.restarts.Add(1)
	}
}

func (s *Supervisor) waitForCondition(ctx context.Context) bool {
	if s.cond == nil {
		return true
	}

	backoff := condInitBackoff
	for {
		ok, reason := s.cond()
		if ok {
			return true
		}
		if reason != "" {
			_, _ = s.ring.Write([]byte("Waiting: " + reason + "\n"))
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > condMaxBackoff {
			backoff = condMaxBackoff
		}
	}
}

func (s *Supervisor) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh, err := s.exe.Run(ctx, s.ring, s.ring)
	if err != nil {
		return fmt.Errorf("starting process: %w", err)
	}

	s.mu.Lock()
	s.cancelFn = cancel
	s.started = time.Now()
	s.mu.Unlock()

	err = <-errCh

	s.mu.Lock()
	s.cancelFn = nil
	s.mu.Unlock()

	return err
}

type lineRingBuffer struct {
	size int

	mu        sync.RWMutex
	remainder string
	r         *ring.Ring
	streams   map[chan string]struct{}
}

func newRingBuffer(size int) *lineRingBuffer {
	return &lineRingBuffer{
		size:    size,
		r:       ring.New(size),
		streams: make(map[chan string]struct{}),
	}
}

func (b *lineRingBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.remainder + string(p)
	for {
		idx := strings.Index(text, "\n")
		if idx == -1 {
			break
		}

		line := text[:idx]
		b.r.Value = line
		for ch := range b.streams {
			select {
			case ch <- line:
			default:
			}
		}
		b.r = b.r.Next()
		text = text[idx+1:]
	}
	b.remainder = text
	return len(p), nil
}

func (b *lineRingBuffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lines := make([]string, 0, b.r.Len())
	b.r.Do(func(v any) {
		if v != nil {
			lines = append(lines, v.(string))
		}
	})
	return lines
}

func (b *lineRingBuffer) Stream(ctx context.Context) <-chan string {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan string, b.size+1)
	b.r.Do(func(x any) {
		if x != nil {
			ch <- x.(string)
		}
	})
	b.streams[ch] = struct{}{}

	go func() {
		<-ctx.Done()

		b.mu.Lock()
		defer b.mu.Unlock()

		delete(b.streams, ch)
		close(ch)
	}()

	return ch
}
