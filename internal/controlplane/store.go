package controlplane

import (
	"context"
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

// Store is the durable backend for the control plane. B1 lifted this
// from a concrete struct to an interface so an external backend
// (Postgres, behind -tags=postgres) can register alongside the built-in
// file/memory store. See RegisterOpener for the dispatch mechanism.
type Store interface {
	Tenant(id string) (TenantRecord, bool)
	PutTenant(record TenantRecord) error
	CredentialRef(id string) (CredentialRef, bool)
	PutCredentialRef(record CredentialRef) error
	Session(id string) (SessionRecord, bool)
	PutSession(record SessionRecord) error
	DeleteSession(id string) error
	AppendAuditEvent(event AuditEvent) error
	// RetainAudit drops audit events older than maxAge and returns the
	// number removed. Called periodically by the retention reaper
	// (B2); maxAge <= 0 is a no-op. Implementations must respect ctx
	// cancellation and must not leave the store in an inconsistent
	// state on partial failure.
	RetainAudit(ctx context.Context, maxAge time.Duration) (int, error)
	// Close releases backend-owned resources (pgxpool, file handles).
	// DevFileStore has nothing to release and returns nil.
	Close() error
}

// Opener is the factory signature registered by external backends.
// Modules that ship under a build tag (e.g. `-tags=postgres`) call
// RegisterOpener from an init() to become dispatchable via Open.
type Opener func(dsn string, opts ...Option) (Store, error)

var (
	openersMu sync.Mutex
	openers   = map[string]Opener{}
)

// RegisterOpener registers an opener for the given DSN scheme. Panics
// on double-register so wiring bugs surface at startup rather than
// silently overwriting a prior registration.
func RegisterOpener(scheme string, fn Opener) {
	if fn == nil {
		panic("controlplane: RegisterOpener called with nil fn")
	}
	openersMu.Lock()
	defer openersMu.Unlock()
	if _, exists := openers[scheme]; exists {
		panic("controlplane: duplicate opener for scheme " + scheme)
	}
	openers[scheme] = fn
}

func lookupOpener(scheme string) (Opener, bool) {
	openersMu.Lock()
	defer openersMu.Unlock()
	fn, ok := openers[scheme]
	return fn, ok
}

// DevFileStore is the file/memory-backed implementation of Store. It
// is the dev/offline fallback; production deployments should register
// an external backend (Postgres) via RegisterOpener and select it
// through the DSN scheme. The C1 guard in cmd/clockify-mcp/main.go
// refuses to start the streamable_http transport against this store
// unless MCP_ALLOW_DEV_BACKEND=1 is set, because the JSON file-rewrite
// write pattern and the in-process mutex do not survive a
// multi-process deployment.
type DevFileStore struct {
	mu    sync.Mutex
	path  string
	state State
	// auditCap, when > 0, caps the in-memory AuditEvents slice so the
	// file-backed store cannot grow unbounded on long-lived dev
	// deployments. On append past the cap the oldest event is dropped
	// (FIFO) before the write. Zero preserves the historical
	// unbounded behaviour for back-compat.
	//
	// B5 rationale: the file store is the dev/offline fallback — the
	// production path is Postgres (B1) with time-based retention (B2).
	// The cap here is the pre-migration safety net for operators
	// running streamable_http on file-backed state.
	auditCap int
}

// Option configures a DevFileStore at construction. External backends
// parse configuration from DSN query parameters, so Options do not
// apply to them.
type Option func(*DevFileStore)

// WithAuditCap caps the in-memory AuditEvents slice at n entries. Zero
// or negative disables the cap. See DevFileStore.auditCap for the
// rationale.
func WithAuditCap(n int) Option {
	return func(s *DevFileStore) {
		if n > 0 {
			s.auditCap = n
		}
	}
}

// Open returns a Store appropriate for the DSN. Built-in schemes:
// "" (alias for memory), "memory", "memory://", "file://<path>", or a
// bare filesystem path — all produce a DevFileStore. Any other scheme
// dispatches to an opener registered via RegisterOpener; if none is
// registered the error explicitly names the scheme and points the
// operator at the matching build tag.
func Open(dsn string, opts ...Option) (Store, error) {
	scheme := dsnScheme(dsn)
	if scheme != "" && scheme != "file" && scheme != "memory" {
		fn, ok := lookupOpener(scheme)
		if !ok {
			return nil, fmt.Errorf("controlplane: unsupported DSN scheme %q; if this is Postgres rebuild the binary with -tags=postgres", scheme)
		}
		return fn(dsn, opts...)
	}
	return openDevFile(dsn, opts...)
}

// dsnScheme extracts the scheme component. Returns "" for the memory
// DSN forms ("" / "memory") and for bare filesystem paths.
func dsnScheme(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" || dsn == "memory" {
		return ""
	}
	scheme, _, ok := strings.Cut(dsn, "://")
	if !ok {
		return ""
	}
	return scheme
}

// Compile-time assertion that DevFileStore satisfies Store. Keeps the
// interface and the file impl from drifting without failing tests.
var _ Store = (*DevFileStore)(nil)

func openDevFile(dsn string, opts ...Option) (Store, error) {
	path, err := resolvePath(dsn)
	if err != nil {
		return nil, err
	}
	s := &DevFileStore{
		path: path,
		state: State{
			Tenants:        map[string]TenantRecord{},
			CredentialRefs: map[string]CredentialRef{},
			Sessions:       map[string]SessionRecord{},
			AuditEvents:    []AuditEvent{},
		},
	}
	for _, o := range opts {
		o(s)
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

func (s *DevFileStore) load() error {
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

func (s *DevFileStore) persistLocked() error {
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

func (s *DevFileStore) Tenant(id string) (TenantRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tenant, ok := s.state.Tenants[id]
	return tenant, ok
}

func (s *DevFileStore) PutTenant(record TenantRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Tenants[record.ID] = record
	return s.persistLocked()
}

func (s *DevFileStore) CredentialRef(id string) (CredentialRef, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.state.CredentialRefs[id]
	return ref, ok
}

func (s *DevFileStore) PutCredentialRef(record CredentialRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CredentialRefs[record.ID] = record
	return s.persistLocked()
}

func (s *DevFileStore) PutSession(record SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Sessions[record.ID] = record
	return s.persistLocked()
}

func (s *DevFileStore) Session(id string) (SessionRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.state.Sessions[id]
	return record, ok
}

func (s *DevFileStore) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.Sessions, id)
	return s.persistLocked()
}

func (s *DevFileStore) AppendAuditEvent(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.AuditEvents = append(s.state.AuditEvents, event)
	if s.auditCap > 0 && len(s.state.AuditEvents) > s.auditCap {
		drop := len(s.state.AuditEvents) - s.auditCap
		// Trim from the front (oldest-first) using a fresh slice so
		// the backing array doesn't keep the dropped entries alive
		// past the next GC.
		kept := make([]AuditEvent, s.auditCap)
		copy(kept, s.state.AuditEvents[drop:])
		s.state.AuditEvents = kept
	}
	return s.persistLocked()
}

// RetainAudit drops audit events older than maxAge from the in-memory
// slice and rewrites the file. Complementary to the WithAuditCap
// hard-limit: the cap protects against bursty writes between reaper
// ticks, retention enforces the time window. maxAge <= 0 is a no-op
// so operators can disable retention by clearing the env var.
func (s *DevFileStore) RetainAudit(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-maxAge)
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]AuditEvent, 0, len(s.state.AuditEvents))
	for _, e := range s.state.AuditEvents {
		if e.At.Before(cutoff) {
			continue
		}
		kept = append(kept, e)
	}
	dropped := len(s.state.AuditEvents) - len(kept)
	if dropped == 0 {
		return 0, nil
	}
	s.state.AuditEvents = kept
	if err := s.persistLocked(); err != nil {
		return 0, err
	}
	return dropped, nil
}

// Close releases backend-owned resources. DevFileStore has no
// goroutines, no connection pool, and no OS handles held between
// calls, so Close is a no-op that always returns nil.
func (s *DevFileStore) Close() error { return nil }
