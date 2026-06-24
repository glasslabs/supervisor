package dbus

import (
	"context"

	"github.com/godbus/dbus/v5"
)

const (
	dbusLogindDest         = "org.freedesktop.login1"
	dbusLogindManagerIface = "org.freedesktop.login1.Manager"
	dbusLogindPath         = "/org/freedesktop/login1"
)

// Logind controls system power state via the systemd-logind D-Bus interface.
// All operations are issued without an authentication prompt.
type Logind interface {
	// PowerOff triggers an immediate system shutdown.
	PowerOff(ctx context.Context) error

	// Reboot triggers an immediate system reboot.
	Reboot(ctx context.Context) error
}

type logind struct {
	obj dbus.BusObject
}

func newLogind(conn *dbus.Conn) Logind {
	return &logind{
		obj: conn.Object(dbusLogindDest, dbusLogindPath),
	}
}

func (l *logind) PowerOff(ctx context.Context) error {
	askForAuth := false
	return l.obj.CallWithContext(ctx, dbusLogindManagerIface+".PowerOff", 0, askForAuth).Err
}

func (l *logind) Reboot(ctx context.Context) error {
	askForAuth := false
	return l.obj.CallWithContext(ctx, dbusLogindManagerIface+".Reboot", 0, askForAuth).Err
}
