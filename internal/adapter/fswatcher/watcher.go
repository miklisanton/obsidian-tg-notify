package fswatcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"obsidian-notify/internal/adapter/config"
)

type SyncService interface {
	SyncFile(ctx context.Context, vault config.VaultConfig, absPath string, now time.Time) error
}

type Watcher struct {
	raw      *fsnotify.Watcher
	vaults   []config.VaultConfig
	syncer   SyncService
	debounce time.Duration
	mu       sync.Mutex
	pending  map[string]pendingSync
}

type pendingSync struct {
	vault config.VaultConfig
	timer *time.Timer
	when  time.Time
}

func New(vaults []config.VaultConfig, syncer SyncService, debounce time.Duration) (*Watcher, error) {
	raw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watcher := &Watcher{raw: raw, vaults: vaults, syncer: syncer, debounce: debounce, pending: make(map[string]pendingSync)}
	for _, vault := range vaults {
		if err := filepath.WalkDir(vault.RootPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsPermission(walkErr) {
					if entry != nil && entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				return walkErr
			}
			if !entry.IsDir() {
				return nil
			}
			if strings.HasPrefix(entry.Name(), ".") {
				if path == vault.RootPath {
					return nil
				}
				return filepath.SkipDir
			}
			return watcher.raw.Add(path)
		}); err != nil {
			return nil, err
		}
	}
	return watcher, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-w.raw.Errors:
			if !ok {
				return nil
			}
			return err
		case event, ok := <-w.raw.Events:
			if !ok {
				return nil
			}
			if err := w.handleEvent(ctx, event); err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) Close() error {
	return w.raw.Close()
}

func (w *Watcher) handleEvent(ctx context.Context, event fsnotify.Event) error {
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			if err := w.raw.Add(event.Name); err != nil {
				return err
			}
			return nil
		}
	}
	if filepath.Ext(event.Name) != ".md" {
		return nil
	}
	if !(event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)) {
		return nil
	}
	vault, ok := w.matchVault(event.Name)
	if !ok {
		return nil
	}
	when := time.Now()
	w.mu.Lock()
	pending, exists := w.pending[event.Name]
	if exists {
		pending.when = when
		pending.vault = vault
		if pending.timer.Stop() {
			pending.timer.Reset(w.debounce)
		} else {
			pending.timer = time.AfterFunc(w.debounce, w.flushFunc(ctx, event.Name))
		}
		w.pending[event.Name] = pending
		w.mu.Unlock()
		return nil
	}
	w.pending[event.Name] = pendingSync{
		vault: vault,
		when:  when,
		timer: time.AfterFunc(w.debounce, w.flushFunc(ctx, event.Name)),
	}
	w.mu.Unlock()
	return nil
}

func (w *Watcher) flushFunc(ctx context.Context, path string) func() {
	return func() {
		w.mu.Lock()
		pending, ok := w.pending[path]
		if ok {
			delete(w.pending, path)
		}
		w.mu.Unlock()
		if !ok || ctx.Err() != nil {
			return
		}
		if err := w.syncer.SyncFile(ctx, pending.vault, path, pending.when); err != nil && !os.IsNotExist(err) {
			log.Printf("sync %s: %v", path, err)
		}
	}
}

func (w *Watcher) matchVault(path string) (config.VaultConfig, bool) {
	for _, vault := range w.vaults {
		if strings.HasPrefix(path, vault.RootPath) {
			return vault, true
		}
	}
	return config.VaultConfig{}, false
}
