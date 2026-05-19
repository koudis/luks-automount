package engine

import "sync"

type DiskState struct {
	mu       sync.Mutex
	DevPath  string
	Unlocked bool
	Mounted  bool
}

func (s *DiskState) Lock()   { s.mu.Lock() }
func (s *DiskState) Unlock() { s.mu.Unlock() }
