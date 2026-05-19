package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/pilebones/go-udev/netlink"
)

const (
	initialBackoff = 500 * time.Millisecond
	maxBackoff     = 30 * time.Second
)

type Monitor struct {
	events chan Event
}

func New() *Monitor {
	return &Monitor{events: make(chan Event, 16)}
}

func (m *Monitor) Events() <-chan Event {
	return m.events
}

func (m *Monitor) Run(ctx context.Context) {
	defer close(m.events)
	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		err := m.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("netlink monitor exited, reconnecting", "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *Monitor) runOnce(ctx context.Context) error {
	conn := &netlink.UEventConn{}
	if err := conn.Connect(netlink.UdevEvent); err != nil {
		return err
	}
	defer conn.Close()

	queue := make(chan netlink.UEvent, 16)
	errs := make(chan error, 1)
	quit := conn.Monitor(queue, errs, nil)

	for {
		select {
		case <-ctx.Done():
			close(quit)
			return nil
		case ev, ok := <-queue:
			if !ok {
				return nil
			}
			m.dispatch(ev)
		case err := <-errs:
			close(quit)
			return err
		}
	}
}

func (m *Monitor) dispatch(ev netlink.UEvent) {
	if ev.Env["SUBSYSTEM"] != "block" {
		return
	}
	if ev.Env["DEVTYPE"] != "disk" {
		return
	}
	devName := ev.Env["DEVNAME"]
	if devName == "" {
		return
	}
	var action Action
	switch ev.Action {
	case netlink.ADD:
		action = ActionAdd
	case netlink.REMOVE:
		action = ActionRemove
	default:
		return
	}
	m.events <- Event{
		Action:  action,
		DevPath: "/dev/" + devName,
		DevName: devName,
	}
}
