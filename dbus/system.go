// Package dbus provides D-Bus clients for system services used by the supervisor.
package dbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// System holds D-Bus clients for system services and owns the underlying connection.
type System struct {
	conn *dbus.Conn

	logind Logind
	rauc   RAUC
}

// New connects to the system D-Bus and returns a System ready for use.
// Call Close when the System is no longer needed.
func New() (*System, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("connecting to system dbus: %w", err)
	}

	return NewWithConn(conn), nil
}

// NewWithConn returns a System using an existing D-Bus connection.
// It is intended for use in tests.
func NewWithConn(conn *dbus.Conn) *System {
	return &System{
		conn:   conn,
		logind: newLogind(conn),
		rauc:   newRAUC(conn),
	}
}

// Logind returns the logind D-Bus client.
func (s *System) Logind() Logind {
	return s.logind
}

// RAUC returns the RAUC D-Bus client.
func (s *System) RAUC() RAUC {
	return s.rauc
}

// Close closes the underlying D-Bus connection.
func (s *System) Close() error {
	return s.conn.Close()
}
