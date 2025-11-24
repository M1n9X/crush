package freshness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Meta tracks access stats for a file path.
type Meta struct {
	Path       string
	SessionID  string
	LastAccess time.Time
	Count      int
}

// Service records "freshness" of files to prioritize context recovery.
type Service struct {
	mu    sync.RWMutex
	store map[string]map[string]*Meta // sessionID -> path -> meta
}

const workspaceSessionID = "_workspace"

// New creates a freshness service.
func New() *Service {
	return &Service{
		store: make(map[string]map[string]*Meta),
	}
}

// Save persists freshness metadata to a file.
func (s *Service) Save(path string) error {
	if path == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all []Meta
	for _, session := range s.store {
		for _, meta := range session {
			all = append(all, *meta)
		}
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Load hydrates freshness metadata from a file.
func (s *Service) Load(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var all []Meta
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = make(map[string]map[string]*Meta)
	for i := range all {
		meta := all[i]
		if meta.SessionID == "" || meta.Path == "" {
			continue
		}
		if _, ok := s.store[meta.SessionID]; !ok {
			s.store[meta.SessionID] = make(map[string]*Meta)
		}
		s.store[meta.SessionID][meta.Path] = &meta
	}
	return nil
}

// Record marks a path as accessed for a given session.
func (s *Service) Record(sessionID, path string) {
	if sessionID == "" || path == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store[sessionID]; !ok {
		s.store[sessionID] = make(map[string]*Meta)
	}
	meta, ok := s.store[sessionID][path]
	if !ok {
		meta = &Meta{Path: path, SessionID: sessionID}
		s.store[sessionID][path] = meta
	}
	meta.Count++
	meta.LastAccess = time.Now()
}

// RecordWorkspace seeds freshness without a concrete session (e.g., config paths).
func (s *Service) RecordWorkspace(path string) {
	s.Record(workspaceSessionID, path)
}

// Top returns up to limit freshest paths for a session, ordered by recency then count.
func (s *Service) Top(sessionID string, limit int) []Meta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	paths := s.store[sessionID]
	if len(paths) == 0 {
		paths = s.store[workspaceSessionID]
	}
	if len(paths) == 0 || limit <= 0 {
		return nil
	}
	list := make([]Meta, 0, len(paths))
	for _, meta := range paths {
		list = append(list, *meta)
	}
	slices.SortFunc(list, func(a, b Meta) int {
		if a.LastAccess.Equal(b.LastAccess) {
			return b.Count - a.Count
		}
		if a.LastAccess.After(b.LastAccess) {
			return -1
		}
		return 1
	})
	if len(list) > limit {
		return list[:limit]
	}
	return list
}
