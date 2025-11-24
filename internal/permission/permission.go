package permission

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

var ErrorPermissionDenied = errors.New("user denied permission")

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionNotification struct {
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type Service interface {
	pubsub.Suscriber[PermissionRequest]
	GrantPersistent(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(opts CreatePermissionRequest) bool
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
	Persistent() []PermissionRequest
	ClearPersistent() error
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	notificationBroker    *pubsub.Broker[PermissionNotification]
	workingDir            string
	sessionPermissions    []PermissionRequest
	sessionPermissionsMu  sync.RWMutex
	pendingRequests       *csync.Map[string, chan bool]
	autoApproveSessions   map[string]bool
	autoApproveSessionsMu sync.RWMutex
	skip                  bool
	allowedTools          []string
	persistentPermissions []PermissionRequest
	persistentPath        string
	persistentMu          sync.RWMutex

	// used to make sure we only process one request at a time
	requestMu     sync.Mutex
	activeRequest *PermissionRequest
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}

	// Persist on disk for the current workspace so approvals survive restarts.
	permission.SessionID = "*" // wildcard to apply across sessions for this project
	s.persistentMu.Lock()
	s.persistentPermissions = append(s.persistentPermissions, permission)
	s.persistLocked()
	s.persistentMu.Unlock()
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
}

func (s *permissionService) Deny(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    false,
		Denied:     true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- false
	}

	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
}

func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	if s.skip {
		return true
	}

	// tell the UI that a permission was requested
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: opts.ToolCallID,
	})
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true
	}

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		return true
	}

	fileInfo, err := os.Stat(opts.Path)
	dir := opts.Path
	if err == nil {
		if fileInfo.IsDir() {
			dir = opts.Path
		} else {
			dir = filepath.Dir(opts.Path)
		}
	}

	if dir == "." {
		dir = s.workingDir
	}
	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			return true
		}
	}
	s.sessionPermissionsMu.RUnlock()

	s.persistentMu.RLock()
	for _, p := range s.persistentPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.Path == permission.Path {
			if p.SessionID == "*" || p.SessionID == permission.SessionID {
				s.persistentMu.RUnlock()
				return true
			}
		}
	}
	s.persistentMu.RUnlock()

	s.activeRequest = &permission

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	return <-respCh
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
}

func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification] {
	return s.notificationBroker.Subscribe(ctx)
}

func (s *permissionService) SetSkipRequests(skip bool) {
	s.skip = skip
}

func (s *permissionService) SkipRequests() bool {
	return s.skip
}

func (s *permissionService) Persistent() []PermissionRequest {
	s.persistentMu.RLock()
	defer s.persistentMu.RUnlock()
	out := make([]PermissionRequest, len(s.persistentPermissions))
	copy(out, s.persistentPermissions)
	return out
}

func (s *permissionService) ClearPersistent() error {
	s.persistentMu.Lock()
	defer s.persistentMu.Unlock()
	s.persistentPermissions = nil
	if s.persistentPath == "" {
		return nil
	}
	return os.Remove(s.persistentPath)
}

func NewPermissionService(workingDir, dataDir string, skip bool, allowedTools []string) Service {
	service := &permissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		notificationBroker:  pubsub.NewBroker[PermissionNotification](),
		workingDir:          workingDir,
		sessionPermissions:  make([]PermissionRequest, 0),
		autoApproveSessions: make(map[string]bool),
		skip:                skip,
		allowedTools:        allowedTools,
		persistentPath:      buildPersistentPath(dataDir, workingDir),
		pendingRequests:     csync.NewMap[string, chan bool](),
	}
	service.loadPersistent()
	return service
}

func buildPersistentPath(dataDir, workingDir string) string {
	return PersistentPath(dataDir, workingDir)
}

// PersistentPath exposes the resolved path for a workspace permission store.
func PersistentPath(dataDir, workingDir string) string {
	if dataDir == "" {
		return ""
	}
	sum := sha1.Sum([]byte(workingDir))
	filename := fmt.Sprintf("%x.json", sum)
	return filepath.Join(dataDir, "permissions", filename)
}

func (s *permissionService) persistLocked() {
	if s.persistentPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.persistentPath), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(s.persistentPermissions, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.persistentPath, data, 0o600)
}

func (s *permissionService) loadPersistent() {
	if s.persistentPath == "" {
		return
	}
	perms, err := LoadPersistent(s.persistentPath)
	if err != nil {
		return
	}
	s.persistentPermissions = perms
}

// LoadPersistent reads a permission store file.
func LoadPersistent(path string) ([]PermissionRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var perms []PermissionRequest
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil, err
	}
	for i := range perms {
		if perms[i].SessionID == "" {
			perms[i].SessionID = "*"
		}
	}
	return perms, nil
}

// SavePersistent writes permission entries to the given path, ensuring directories exist.
func SavePersistent(path string, perms []PermissionRequest) error {
	if path == "" {
		return fmt.Errorf("permission store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(perms, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
