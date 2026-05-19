package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"luks-automount/internal/config"
	"luks-automount/internal/keyring"
	"luks-automount/internal/luks"
	"luks-automount/internal/monitor"
	"luks-automount/internal/worker"
)

const shutdownDrain = 10 * time.Second

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
	uuid, err := luks.ReadUUID(ev.DevPath)
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
	if err := e.client.UnlockAndMount(req, passBytes); err != nil {
		slog.Error("unlock+mount failed", "name", disk.Name, "err", err)
		return
	}
	state.Unlocked = true
	state.Mounted = true
	slog.Info("disk unlocked and mounted", "name", disk.Name, "dev", ev.DevPath, "mount", disk.MountPoint)
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

	if state.Mounted {
		slog.Warn("device removed while mounted", "name", disk.Name, "dev", ev.DevPath)
	}
	req := &worker.Request{
		Op:         worker.OpUnmountAndClose,
		Mapper:     disk.MapperName,
		MountPoint: disk.MountPoint,
	}
	if err := e.client.UnmountAndClose(req); err != nil {
		slog.Error("unmount+close failed", "name", disk.Name, "err", err)
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

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
