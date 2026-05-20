package monitor

import (
	"testing"

	"github.com/pilebones/go-udev/netlink"
)

func TestDispatchAcceptsShortDevName(t *testing.T) {
	m := New()
	m.dispatch(blockEvent(netlink.ADD, "sdb"))

	ev := <-m.Events()
	if ev.Action != ActionAdd || ev.DevPath != "/dev/sdb" || ev.DevName != "sdb" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestDispatchAcceptsFullDevName(t *testing.T) {
	m := New()
	m.dispatch(blockEvent(netlink.ADD, "/dev/sdb"))

	ev := <-m.Events()
	if ev.Action != ActionAdd || ev.DevPath != "/dev/sdb" || ev.DevName != "sdb" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestDispatchAcceptsPartition(t *testing.T) {
	m := New()
	m.dispatch(blockEventWithType(netlink.ADD, "/dev/sdb1", "partition"))

	ev := <-m.Events()
	if ev.Action != ActionAdd || ev.DevPath != "/dev/sdb1" || ev.DevName != "sdb1" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestDispatchDropsDuplicateEvents(t *testing.T) {
	m := New()
	m.dispatch(blockEvent(netlink.ADD, "/dev/sdb"))
	m.dispatch(blockEvent(netlink.ADD, "/dev/sdb"))

	<-m.Events()
	select {
	case ev := <-m.Events():
		t.Fatalf("unexpected duplicate event: %+v", ev)
	default:
	}
}

func TestDispatchKeepsDifferentActions(t *testing.T) {
	m := New()
	m.dispatch(blockEvent(netlink.ADD, "/dev/sdb"))
	m.dispatch(blockEvent(netlink.REMOVE, "/dev/sdb"))

	<-m.Events()
	ev := <-m.Events()
	if ev.Action != ActionRemove || ev.DevPath != "/dev/sdb" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func blockEvent(action netlink.KObjAction, devName string) netlink.UEvent {
	return blockEventWithType(action, devName, "disk")
}

func blockEventWithType(action netlink.KObjAction, devName string, devType string) netlink.UEvent {
	return netlink.UEvent{
		Action: action,
		Env: map[string]string{
			"SUBSYSTEM": "block",
			"DEVTYPE":   devType,
			"DEVNAME":   devName,
		},
	}
}
