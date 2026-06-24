package dbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	dbusRAUCDest  = "de.pengutronix.rauc"
	dbusRAUCIface = "de.pengutronix.rauc.Installer"
	dbusRAUCPath  = "/"
)

// RAUCSlot describes the state of a single RAUC slot.
type RAUCSlot struct {
	Name     string `json:"name"`
	Class    string `json:"class"`
	Device   string `json:"device"`
	Type     string `json:"type"`
	Bootname string `json:"bootname"`
	State    string `json:"state"`
	SHA256   string `json:"sha256,omitempty"`
	Size     uint64 `json:"size,omitempty"`
}

// RAUCStatus describes the overall system status reported by RAUC.
type RAUCStatus struct {
	Compatible string     `json:"compatible"`
	Variant    string     `json:"variant"`
	BootSlot   string     `json:"booted"`
	Slots      []RAUCSlot `json:"slots"`
}

// RAUC manages OS bundle installation via the RAUC D-Bus interface.
type RAUC interface {
	// Status returns the current system compatibility, boot slot, and the
	// state of all configured slots.
	Status(ctx context.Context) (RAUCStatus, error)

	// Install installs the bundle at filename. It blocks until RAUC signals
	// completion, so the caller should pass a context unaffected by client
	// disconnection if the installation must not be interrupted.
	Install(ctx context.Context, filename string) error
}

type rauc struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

func newRAUC(conn *dbus.Conn) RAUC {
	return &rauc{
		conn: conn,
		obj:  conn.Object(dbusRAUCDest, dbusRAUCPath),
	}
}

func (r *rauc) Status(ctx context.Context) (RAUCStatus, error) {
	var compatible, variant, bootSlot string
	if err := r.obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		dbusRAUCIface, "Compatible").Store(&compatible); err != nil {
		return RAUCStatus{}, fmt.Errorf("reading Compatible: %w", err)
	}
	if err := r.obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		dbusRAUCIface, "Variant").Store(&variant); err != nil {
		return RAUCStatus{}, fmt.Errorf("reading Variant: %w", err)
	}
	if err := r.obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		dbusRAUCIface, "BootSlot").Store(&bootSlot); err != nil {
		return RAUCStatus{}, fmt.Errorf("reading BootSlot: %w", err)
	}

	var rawSlots [][]any
	if err := r.obj.CallWithContext(ctx, dbusRAUCIface+".GetSlotStatus", 0).Store(&rawSlots); err != nil {
		return RAUCStatus{}, fmt.Errorf("getting slot status: %w", err)
	}

	slots := make([]RAUCSlot, 0, len(rawSlots))
	for _, entry := range rawSlots {
		if len(entry) < 2 {
			continue
		}
		name, _ := entry[0].(string)
		props, _ := entry[1].(map[string]dbus.Variant)

		slot := RAUCSlot{Name: name}
		if v, ok := props["class"]; ok {
			slot.Class, _ = v.Value().(string)
		}
		if v, ok := props["device"]; ok {
			slot.Device, _ = v.Value().(string)
		}
		if v, ok := props["type"]; ok {
			slot.Type, _ = v.Value().(string)
		}
		if v, ok := props["bootname"]; ok {
			slot.Bootname, _ = v.Value().(string)
		}
		if v, ok := props["state"]; ok {
			slot.State, _ = v.Value().(string)
		}
		if v, ok := props["sha256"]; ok {
			slot.SHA256, _ = v.Value().(string)
		}
		if v, ok := props["size"]; ok {
			slot.Size, _ = v.Value().(uint64)
		}
		slots = append(slots, slot)
	}

	return RAUCStatus{
		Compatible: compatible,
		Variant:    variant,
		BootSlot:   bootSlot,
		Slots:      slots,
	}, nil
}

func (r *rauc) Install(ctx context.Context, filename string) error {
	// Subscribe to the Completed signal before calling Install to avoid a race.
	ch := make(chan *dbus.Signal, 1)
	r.conn.Signal(ch)
	defer r.conn.RemoveSignal(ch)

	if err := r.conn.AddMatchSignal(
		dbus.WithMatchInterface(dbusRAUCIface),
		dbus.WithMatchMember("Completed"),
	); err != nil {
		return fmt.Errorf("subscribing to Completed signal: %w", err)
	}
	defer func() {
		_ = r.conn.RemoveMatchSignal(
			dbus.WithMatchInterface(dbusRAUCIface),
			dbus.WithMatchMember("Completed"),
		)
	}()

	installCtx := context.WithoutCancel(ctx)
	if err := r.obj.CallWithContext(installCtx, dbusRAUCIface+".Install", 0, filename).Err; err != nil {
		return fmt.Errorf("calling Install: %w", err)
	}

	select {
	case sig := <-ch:
		if len(sig.Body) == 0 {
			return nil
		}
		result, _ := sig.Body[0].(int32)
		if result != 0 {
			var lastErr string
			_ = r.obj.CallWithContext(installCtx, "org.freedesktop.DBus.Properties.Get", 0,
				dbusRAUCIface, "LastError").Store(&lastErr)
			if lastErr != "" {
				return fmt.Errorf("installation failed: %s", lastErr)
			}
			return fmt.Errorf("installation failed with result %d", result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
