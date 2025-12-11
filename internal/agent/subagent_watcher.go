package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// SubAgentWatcher watches for changes in subagent configuration directories
// and automatically clears the cache when changes are detected.
type SubAgentWatcher struct {
	watcher  *fsnotify.Watcher
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	onChange func()
}

// NewSubAgentWatcher creates a new file watcher for subagent definitions.
func NewSubAgentWatcher(ctx context.Context, workingDir string, onChange func()) (*SubAgentWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watcherCtx, cancel := context.WithCancel(ctx)

	w := &SubAgentWatcher{
		watcher:  watcher,
		ctx:      watcherCtx,
		cancel:   cancel,
		done:     make(chan struct{}),
		onChange: onChange,
	}

	// Add directories to watch
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".claude", "agents"),
		filepath.Join(home, ".codebreeze", "agents"),
		filepath.Join(home, ".crush", "agents"),
		filepath.Join(workingDir, ".claude", "agents"),
		filepath.Join(workingDir, ".codebreeze", "agents"),
		filepath.Join(workingDir, ".crush", "agents"),
	}

	for _, path := range paths {
		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			if err := watcher.Add(path); err != nil {
				slog.Warn("Failed to watch subagent directory", "path", path, "error", err)
			} else {
				slog.Debug("Watching subagent directory", "path", path)
			}
		}
	}

	return w, nil
}

// Start begins watching for file changes in the background.
func (w *SubAgentWatcher) Start() {
	go w.watch()
}

// Stop stops the watcher and releases resources.
func (w *SubAgentWatcher) Stop() {
	w.cancel()
	<-w.done
}

// watch is the main event loop that processes file system events.
func (w *SubAgentWatcher) watch() {
	defer close(w.done)
	defer w.watcher.Close()

	for {
		select {
		case <-w.ctx.Done():
			slog.Debug("Subagent watcher stopped")
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only react to .md files
			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}

			// Handle create, write, rename, and remove events
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) ||
				event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				slog.Debug("Subagent configuration changed", "file", event.Name, "op", event.Op)
				clearSubAgentDefsCache()
				if w.onChange != nil {
					go w.onChange()
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("Subagent watcher error", "error", err)
		}
	}
}
