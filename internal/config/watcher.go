package config

import (
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// WatchCallback is called when a config file changes.
type WatchCallback func(path string)

// Watcher monitors config files for changes and triggers hot-reload.
type Watcher struct {
	watcher  *fsnotify.Watcher
	callback WatchCallback
	done     chan struct{}
}

// NewWatcher creates a config file watcher.
func NewWatcher(dirs []string, callback WatchCallback) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if err := w.Add(dir); err != nil {
			slog.Warn("Could not watch directory", "dir", dir, "error", err)
		}
	}

	cw := &Watcher{
		watcher:  w,
		callback: callback,
		done:     make(chan struct{}),
	}

	go cw.run()
	return cw, nil
}

func (cw *Watcher) run() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				ext := filepath.Ext(event.Name)
				if ext == ".yaml" || ext == ".yml" {
					slog.Info("Config file changed", "path", event.Name)
					cw.callback(event.Name)
				}
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Config watcher error", "error", err)

		case <-cw.done:
			return
		}
	}
}

// Close stops the watcher.
func (cw *Watcher) Close() error {
	close(cw.done)
	return cw.watcher.Close()
}
