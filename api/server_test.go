package api_test

import (
	"context"
	"io"

	"github.com/glasslabs/supervisor/api"
	"github.com/glasslabs/supervisor/dbus"
	"github.com/glasslabs/supervisor/proc"
	"github.com/hamba/logger/v2"
	"github.com/stretchr/testify/mock"
)

type mockSupervisor struct {
	mock.Mock
}

func (m *mockSupervisor) Restart() {
	m.Called()
}

func (m *mockSupervisor) Info() proc.Info {
	args := m.Called()
	return args.Get(0).(proc.Info)
}

func (m *mockSupervisor) Lines() []string {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]string)
	}
	return nil
}

func (m *mockSupervisor) Follow(_ context.Context) <-chan string {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.(<-chan string)
	}
	ch := make(chan string)
	close(ch)
	return ch
}

type system struct {
	logind *mockLogind
	rauc   *mockRAUC
}

func (s system) Logind() dbus.Logind {
	return s.logind
}

func (s system) RAUC() dbus.RAUC {
	return s.rauc
}

type mockLogind struct {
	mock.Mock
}

func (m *mockLogind) PowerOff(_ context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockLogind) Reboot(_ context.Context) error {
	args := m.Called()
	return args.Error(0)
}

type mockRAUC struct {
	mock.Mock
}

func (m *mockRAUC) Status(_ context.Context) (dbus.RAUCStatus, error) {
	args := m.Called()
	return args.Get(0).(dbus.RAUCStatus), args.Error(1)
}

func (m *mockRAUC) Install(_ context.Context, filename string) error {
	args := m.Called(filename)
	return args.Error(0)
}

func newServer(sup api.Supervisor, rauc api.System, glassBin, dataDir string) *api.Server {
	log := logger.New(io.Discard, logger.LogfmtFormat(), logger.Error)

	return api.NewServer("", sup, rauc, glassBin, dataDir, log)
}
