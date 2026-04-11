package controlplane

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TenantRecord struct {
	ID              string            `json:"id"`
	CredentialRefID string            `json:"credentialRefId"`
	WorkspaceID     string            `json:"workspaceId,omitempty"`
	BaseURL         string            `json:"baseUrl,omitempty"`
	Timezone        string            `json:"timezone,omitempty"`
	PolicyMode      string            `json:"policyMode,omitempty"`
	DenyTools       []string          `json:"denyTools,omitempty"`
	DenyGroups      []string          `json:"denyGroups,omitempty"`
	AllowGroups     []string          `json:"allowGroups,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type CredentialRef struct {
	ID         string            `json:"id"`
	Backend    string            `json:"backend"`
	Reference  string            `json:"reference"`
	Workspace  string            `json:"workspaceId,omitempty"`
	BaseURL    string            `json:"baseUrl,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	ModifiedAt time.Time         `json:"modifiedAt,omitempty"`
}

type SessionRecord struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenantId"`
	Subject           string    `json:"subject"`
	Transport         string    `json:"transport"`
	ProtocolVersion   string    `json:"protocolVersion,omitempty"`
	ClientName        string    `json:"clientName,omitempty"`
	ClientVersion     string    `json:"clientVersion,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	LastSeenAt        time.Time `json:"lastSeenAt"`
	WorkspaceID       string    `json:"workspaceId,omitempty"`
	ClockifyBaseURL   string    `json:"clockifyBaseUrl,omitempty"`
	SessionAffinityID string    `json:"sessionAffinityId,omitempty"`
}

type AuditEvent struct {
	ID          string            `json:"id"`
	At          time.Time         `json:"at"`
	TenantID    string            `json:"tenantId"`
	Subject     string            `json:"subject"`
	SessionID   string            `json:"sessionId"`
	Transport   string            `json:"transport,omitempty"`
	Tool        string            `json:"tool,omitempty"`
	Action      string            `json:"action"`
	Outcome     string            `json:"outcome"`
	Reason      string            `json:"reason,omitempty"`
	ResourceIDs map[string]string `json:"resourceIds,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type State struct {
	Tenants        map[string]TenantRecord  `json:"tenants"`
	CredentialRefs map[string]CredentialRef `json:"credential_refs"`
	Sessions       map[string]SessionRecord `json:"sessions"`
	AuditEvents    []AuditEvent             `json:"audit_events"`
}

type Store struct {
	mu    sync.Mutex
	path  string
	state State
}

func Open(dsn string) (*Store, error) {
	path, err := resolvePath(dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{
		path: path,
		state: State{
			Tenants:        map[string]TenantRecord{},
			CredentialRefs: map[string]CredentialRef{},
			Sessions:       map[string]SessionRecord{},
			AuditEvents:    []AuditEvent{},
		},
	}
	if path == "" {
		return s, nil
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func resolvePath(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" || dsn == "memory" || dsn == "memory://" {
		return "", nil
	}
	if strings.HasPrefix(dsn, "file://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("invalid control-plane DSN: %w", err)
		}
		if u.Path == "" {
			return "", fmt.Errorf("file DSN must include a path")
		}
		return u.Path, nil
	}
	if strings.Contains(dsn, "://") {
		return "", fmt.Errorf("unsupported control-plane DSN %q", dsn)
	}
	return dsn, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read control-plane store: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, &s.state); err != nil {
		return fmt.Errorf("decode control-plane store: %w", err)
	}
	if s.state.Tenants == nil {
		s.state.Tenants = map[string]TenantRecord{}
	}
	if s.state.CredentialRefs == nil {
		s.state.CredentialRefs = map[string]CredentialRef{}
	}
	if s.state.Sessions == nil {
		s.state.Sessions = map[string]SessionRecord{}
	}
	if s.state.AuditEvents == nil {
		s.state.AuditEvents = []AuditEvent{}
	}
	return nil
}

func (s *Store) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir control-plane dir: %w", err)
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode control-plane store: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0o600); err != nil {
		return fmt.Errorf("write control-plane store: %w", err)
	}
	return nil
}

func (s *Store) Tenant(id string) (TenantRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tenant, ok := s.state.Tenants[id]
	return tenant, ok
}

func (s *Store) PutTenant(record TenantRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Tenants[record.ID] = record
	return s.persistLocked()
}

func (s *Store) CredentialRef(id string) (CredentialRef, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.state.CredentialRefs[id]
	return ref, ok
}

func (s *Store) PutCredentialRef(record CredentialRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CredentialRefs[record.ID] = record
	return s.persistLocked()
}

func (s *Store) PutSession(record SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Sessions[record.ID] = record
	return s.persistLocked()
}

func (s *Store) Session(id string) (SessionRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.state.Sessions[id]
	return record, ok
}

func (s *Store) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.Sessions, id)
	return s.persistLocked()
}

func (s *Store) AppendAuditEvent(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.AuditEvents = append(s.state.AuditEvents, event)
	return s.persistLocked()
}
