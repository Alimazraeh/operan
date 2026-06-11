// Package persist snapshots in-memory stores to JSON files on disk so
// state survives pod restarts (backed by a hostPath volume in k8s).
// Snapshots are periodic plus once on shutdown; at most one interval of
// data is lost on a crash.
package persist

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Store is anything that can dump and restore its full state as JSON.
type Store interface {
	Export() ([]byte, error)
	Import([]byte) error
}

// File binds a store to its snapshot file name within the data dir.
type File struct {
	Name  string
	Store Store
}

// Load restores every store from its snapshot, best effort: a missing
// file is a fresh start, a corrupt one is logged and skipped.
func Load(dir string, files []File) {
	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("[PERSIST] read %s: %v", path, err)
			}
			continue
		}
		if err := f.Store.Import(data); err != nil {
			log.Printf("[PERSIST] restore %s: %v", path, err)
			continue
		}
		log.Printf("[PERSIST] restored %s (%d bytes)", f.Name, len(data))
	}
}

// SaveAll snapshots every store atomically (tmp file + rename).
func SaveAll(dir string, files []File) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[PERSIST] mkdir %s: %v", dir, err)
		return
	}
	for _, f := range files {
		data, err := f.Store.Export()
		if err != nil {
			log.Printf("[PERSIST] export %s: %v", f.Name, err)
			continue
		}
		path := filepath.Join(dir, f.Name)
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			log.Printf("[PERSIST] write %s: %v", tmp, err)
			continue
		}
		if err := os.Rename(tmp, path); err != nil {
			log.Printf("[PERSIST] rename %s: %v", path, err)
		}
	}
}

// Run saves on a ticker until ctx is cancelled, then saves one final time.
func Run(ctx context.Context, dir string, interval time.Duration, files []File) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			SaveAll(dir, files)
		case <-ctx.Done():
			SaveAll(dir, files)
			return
		}
	}
}
