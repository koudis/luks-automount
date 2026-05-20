package monitor

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/pilebones/go-udev/netlink"
)

const (
	initialBackoff      = 500 * time.Millisecond
	maxBackoff          = 30 * time.Second
	duplicateEventDelay = 2 * time.Second
	eventMode           = netlink.Mode(int(netlink.KernelEvent) | int(netlink.UdevEvent))
)

type Monitor struct {
	events       chan Event
	recentEvents map[string]time.Time
}

func New() *Monitor {
	return &Monitor{
		events:       make(chan Event, 16),
		recentEvents: make(map[string]time.Time),
	}
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
	if err := conn.Connect(eventMode); err != nil {
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
	if !isBlockDeviceType(ev.Env["DEVTYPE"]) {
		return
	}
	devName := ev.Env["DEVNAME"]
	if devName == "" {
		return
	}
	devPath := devPathFromName(devName)
	var action Action
	switch ev.Action {
	case netlink.ADD:
		action = ActionAdd
	case netlink.REMOVE:
		action = ActionRemove
	default:
		return
	}
	if m.isDuplicate(action, devPath) {
		return
	}
	m.events <- Event{
		Action:  action,
		DevPath: devPath,
		DevName: filepath.Base(devPath),
	}
}

func devPathFromName(devName string) string {
	if strings.HasPrefix(devName, "/dev/") {
		return devName
	}
	return "/dev/" + devName
}

func isBlockDeviceType(devType string) bool {
	return devType == "disk" || devType == "partition"
}

func (m *Monitor) isDuplicate(action Action, devPath string) bool {
	key := action.String() + "\x00" + devPath
	now := time.Now()
	if last, ok := m.recentEvents[key]; ok && now.Sub(last) < duplicateEventDelay {
		return true
	}
	m.recentEvents[key] = now
	return false
}
