package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"luks-automount/internal/config"
	"luks-automount/internal/dialog"
	"luks-automount/internal/keyring"
	"luks-automount/internal/monitor"
	"luks-automount/internal/worker"
)

const (
	shutdownDrain    = 10 * time.Second
	readUUIDAttempts = 10
)

var (
	readUUID           = (*worker.Client).ReadUUID
	unlockAndMount     = (*worker.Client).UnlockAndMount
	unmountAndClose    = (*worker.Client).UnmountAndClose
	showMountPointBusy = dialog.ShowMountPointBusy
	readUUIDRetryDelay = 500 * time.Millisecond
)

type Engine struct {
	cfg    *config.Config
	mon    *monitor.Monitor
	client *worker.Client
	uid    int
	gid    int

	mu      sync.Mutex
	states  map[string]*DiskState
	devUUID map[string]string
}

func New(cfg *config.Config, client *worker.Client) *Engine {
	return &Engine{
		cfg:     cfg,
		mon:     monitor.New(),
		client:  client,
		uid:     os.Getuid(),
		gid:     os.Getgid(),
		states:  make(map[string]*DiskState),
		devUUID: make(map[string]string),
	}
}

func (e *Engine) Run(ctx context.Context) error {
	var monWG sync.WaitGroup
	monWG.Add(1)
	go func() {
		defer monWG.Done()
		e.mon.Run(ctx)
	}()

	e.scanExisting()

	var workWG sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			e.drain(&workWG)
			monWG.Wait()
			return nil
		case ev, ok := <-e.mon.Events():
			if !ok {
				e.drain(&workWG)
				monWG.Wait()
				return nil
			}
			workWG.Add(1)
			go func(ev monitor.Event) {
				defer workWG.Done()
				e.handleEvent(ev)
			}(ev)
		}
	}
}

func (e *Engine) drain(wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownDrain):
		slog.Warn("shutdown drain timeout exceeded", "timeout", shutdownDrain)
	}
}

func (e *Engine) scanExisting() {
	disks, err := ScanPluggedDisks()
	if err != nil {
		slog.Warn("scan existing disks failed", "err", err)
		return
	}
	for _, d := range disks {
		go e.handleEvent(monitor.Event{
			Action:  monitor.ActionAdd,
			DevPath: d.DevPath,
			DevName: d.DevName,
		})
	}
}

func (e *Engine) handleEvent(ev monitor.Event) {
	switch ev.Action {
	case monitor.ActionAdd:
		e.handleAdd(ev)
	case monitor.ActionRemove:
		e.handleRemove(ev)
	}
}

func (e *Engine) handleAdd(ev monitor.Event) {
	uuid, err := readUUIDWithRetry(e.client, ev.DevPath)
	if err != nil {
		return
	}
	disk := e.cfg.FindByUUID(uuid)
	if disk == nil {
		return
	}
	state := e.stateFor(uuid)
	state.Lock()
	defer state.Unlock()

	if state.Unlocked && state.Mounted {
		slog.Info("disk already unlocked and mounted", "name", disk.Name)
		return
	}
	state.DevPath = ev.DevPath
	e.rememberDev(ev.DevPath, uuid)
	if state.Unlocked {
		req := &worker.Request{
			Op:         worker.OpUnmountAndClose,
			Mapper:     disk.MapperName,
			MountPoint: disk.MountPoint,
		}
		if err := unmountAndClose(e.client, req); err != nil {
			reportMountPointBusy(disk.MountPoint, err)
			slog.Error("unmount+close failed", "name", disk.Name, "err", err)
			return
		}
		state.Unlocked = false
		state.Mounted = false
	}

	pass, err := keyring.Get(disk.Name)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			slog.Warn("keyring entry missing", "name", disk.Name)
			return
		}
		slog.Error("keyring lookup failed", "name", disk.Name, "err", err)
		return
	}
	passBytes := []byte(pass)
	defer zero(passBytes)

	req := &worker.Request{
		Op:         worker.OpUnlockAndMount,
		Dev:        ev.DevPath,
		Mapper:     disk.MapperName,
		MountPoint: disk.MountPoint,
		FS:         disk.FilesystemType,
		Options:    disk.MountOptions,
		UID:        e.uid,
		GID:        e.gid,
	}
	if err := unlockAndMount(e.client, req, passBytes); err != nil {
		slog.Error("unlock+mount failed", "name", disk.Name, "err", err)
		return
	}
	state.Unlocked = true
	state.Mounted = true
	slog.Info("disk unlocked and mounted", "name", disk.Name, "dev", ev.DevPath, "mount", disk.MountPoint)
}

func readUUIDWithRetry(client *worker.Client, devPath string) (string, error) {
	var err error
	for attempt := 0; attempt < readUUIDAttempts; attempt++ {
		var uuid string
		uuid, err = readUUID(client, devPath)
		if err == nil {
			return uuid, nil
		}
		if attempt == readUUIDAttempts-1 {
			break
		}
		time.Sleep(readUUIDRetryDelay)
	}
	return "", err
}

func (e *Engine) handleRemove(ev monitor.Event) {
	uuid := e.lookupDev(ev.DevPath)
	if uuid == "" {
		return
	}
	disk := e.cfg.FindByUUID(uuid)
	if disk == nil {
		return
	}
	state := e.stateFor(uuid)
	state.Lock()
	defer state.Unlock()
	if state.DevPath != ev.DevPath {
		return
	}

	if state.Mounted {
		slog.Warn("device removed while mounted", "name", disk.Name, "dev", ev.DevPath)
	}
	req := &worker.Request{
		Op:         worker.OpUnmountAndClose,
		Mapper:     disk.MapperName,
		MountPoint: disk.MountPoint,
	}
	if err := unmountAndClose(e.client, req); err != nil {
		reportMountPointBusy(disk.MountPoint, err)
		slog.Error("unmount+close failed", "name", disk.Name, "err", err)
		state.Mounted = false
		state.DevPath = ""
		e.forgetDev(ev.DevPath)
		return
	} else {
		slog.Info("disk unmounted and closed", "name", disk.Name)
	}
	state.Unlocked = false
	state.Mounted = false
	state.DevPath = ""
	e.forgetDev(ev.DevPath)
}

func (e *Engine) stateFor(uuid string) *DiskState {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.states[uuid]
	if !ok {
		s = &DiskState{}
		e.states[uuid] = s
	}
	return s
}

func (e *Engine) rememberDev(devPath, uuid string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.devUUID[devPath] = uuid
}

func (e *Engine) lookupDev(devPath string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.devUUID[devPath]
}

func (e *Engine) forgetDev(devPath string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.devUUID, devPath)
}

func reportMountPointBusy(mountPoint string, err error) {
	var busy *worker.MountPointBusyError
	if !errors.As(err, &busy) {
		return
	}
	if dialogErr := showMountPointBusy(mountPoint, busy.Users); dialogErr != nil {
		slog.Warn("mount-point-busy dialog failed", "mount_point", mountPoint, "err", dialogErr)
	}
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
