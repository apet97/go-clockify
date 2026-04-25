//go:build postgres && integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/apet97/go-clockify/internal/controlplane"
	_ "github.com/apet97/go-clockify/internal/controlplane/postgres" // register opener
)

var (
	sharedDSNOnce sync.Once
	sharedDSN     string
	sharedCleanup func()
	sharedErr     error
)

// dsn lazily starts a postgres:16 container and returns its connection
// string. One container is reused across every test in the package so
// the 8–15 second pull+start cost is paid once. TestMain calls cleanup.
//
// When the container fails to start (Docker daemon missing, image pull
// blocked, network unreachable), the default behaviour is t.Skip so a
// laptop without Docker still builds and runs the rest of the suite.
// CI must instead fail loudly so a regression in the integration gate
// does not ship as green: set INTEGRATION_REQUIRED=1 in the environment
// and dsn() will t.Fatalf on Testcontainers failure. The Makefile's
// test-postgres target sets the env var; the same flag should be set
// in any CI workflow that invokes the integration suite on main.
func dsn(t *testing.T) string {
	t.Helper()
	sharedDSNOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		pg, err := tcpostgres.Run(ctx,
			"postgres:16-alpine",
			tcpostgres.WithDatabase("controlplane_test"),
			tcpostgres.WithUsername("cp"),
			tcpostgres.WithPassword("cp"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(90*time.Second)),
		)
		if err != nil {
			sharedErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		sharedCleanup = func() {
			_ = pg.Terminate(context.Background())
		}
		connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			sharedErr = fmt.Errorf("connection string: %w", err)
			return
		}
		sharedDSN = connStr
	})
	if sharedErr != nil {
		if integrationRequired() {
			t.Fatalf("postgres integration tests required (INTEGRATION_REQUIRED=1) but Testcontainers failed: %v", sharedErr)
		}
		t.Skipf("postgres unavailable: %v", sharedErr)
	}
	return sharedDSN
}

// integrationRequired returns true when callers want a Testcontainers
// failure to surface as t.Fatal rather than t.Skip. Used by CI and
// `make test-postgres` to ensure the integration gate cannot pass
// vacuously when Docker is unavailable.
func integrationRequired() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("INTEGRATION_REQUIRED")))
	return v == "1" || v == "true" || v == "yes"
}

// TestIntegrationRequiredEnvGate exercises the env-var parsing in
// integrationRequired() so a regression that breaks the gate
// (e.g. a typo, or expanding the env name) surfaces immediately
// rather than waiting for a Docker-less CI run that would silently
// fall back to skip-green. The Testcontainers-failure → t.Fatal
// pathway in dsn() relies on this helper, so guarding the helper is
// the cheapest way to anchor the gate's semantics.
func TestIntegrationRequiredEnvGate(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"no", false},
		{"foo", false},
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"  yes  ", true},
	}
	for _, tc := range cases {
		t.Run("env="+tc.env, func(t *testing.T) {
			t.Setenv("INTEGRATION_REQUIRED", tc.env)
			if got := integrationRequired(); got != tc.want {
				t.Errorf("integrationRequired() with env=%q = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedCleanup != nil {
		sharedCleanup()
	}
	os.Exit(code)
}

func openStore(t *testing.T) controlplane.Store {
	t.Helper()
	s, err := controlplane.Open(dsn(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestRoundTrip_AllRecords writes one of each record type and reads it
// back, asserting every field survives the JSONB / timestamp mapping.
func TestRoundTrip_AllRecords(t *testing.T) {
	s := openStore(t)

	ref := controlplane.CredentialRef{
		ID:         "cred-rt",
		Backend:    "env",
		Reference:  "API_KEY",
		Workspace:  "ws-rt",
		BaseURL:    "https://api.example.com",
		Metadata:   map[string]string{"owner": "alice"},
		ModifiedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.PutCredentialRef(ref); err != nil {
		t.Fatalf("PutCredentialRef: %v", err)
	}
	gotRef, ok := s.CredentialRef(ref.ID)
	if !ok {
		t.Fatal("CredentialRef missing after Put")
	}
	if gotRef.Reference != "API_KEY" || gotRef.Metadata["owner"] != "alice" {
		t.Fatalf("CredentialRef round-trip lost fields: %+v", gotRef)
	}
	if !gotRef.ModifiedAt.Equal(ref.ModifiedAt) {
		t.Fatalf("ModifiedAt lost: want %v got %v", ref.ModifiedAt, gotRef.ModifiedAt)
	}

	tenant := controlplane.TenantRecord{
		ID:              "tenant-rt",
		CredentialRefID: "cred-rt",
		WorkspaceID:     "ws-rt",
		BaseURL:         "https://api.example.com",
		Timezone:        "UTC",
		PolicyMode:      "standard",
		DenyTools:       []string{"clockify_delete_entry"},
		AllowGroups:     []string{"read-only"},
		Metadata:        map[string]string{"team": "infra"},
	}
	if err := s.PutTenant(tenant); err != nil {
		t.Fatalf("PutTenant: %v", err)
	}
	gotTenant, ok := s.Tenant(tenant.ID)
	if !ok {
		t.Fatal("Tenant missing after Put")
	}
	if len(gotTenant.DenyTools) != 1 || gotTenant.DenyTools[0] != "clockify_delete_entry" {
		t.Fatalf("DenyTools lost: %+v", gotTenant.DenyTools)
	}
	if gotTenant.Metadata["team"] != "infra" {
		t.Fatalf("Metadata lost: %+v", gotTenant.Metadata)
	}

	now := time.Now().UTC().Truncate(time.Second)
	session := controlplane.SessionRecord{
		ID:              "sess-rt",
		TenantID:        tenant.ID,
		Subject:         "alice@example.com",
		Transport:       "streamable_http",
		ProtocolVersion: "2025-06-18",
		CreatedAt:       now,
		ExpiresAt:       now.Add(30 * time.Minute),
		LastSeenAt:      now,
	}
	if err := s.PutSession(session); err != nil {
		t.Fatalf("PutSession: %v", err)
	}
	gotSess, ok := s.Session(session.ID)
	if !ok {
		t.Fatal("Session missing after Put")
	}
	if !gotSess.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("ExpiresAt lost: want %v got %v", session.ExpiresAt, gotSess.ExpiresAt)
	}
	if err := s.DeleteSession(session.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, ok := s.Session(session.ID); ok {
		t.Fatal("Session still present after Delete")
	}

	for i := 0; i < 3; i++ {
		err := s.AppendAuditEvent(controlplane.AuditEvent{
			ID:        fmt.Sprintf("audit-%d", i),
			At:        now.Add(time.Duration(i) * time.Second),
			TenantID:  tenant.ID,
			Subject:   "alice@example.com",
			SessionID: session.ID,
			Tool:      "clockify_log_time",
			Action:    "tools/call",
			Outcome:   "success",
			Metadata:  map[string]string{"seq": fmt.Sprintf("%d", i)},
		})
		if err != nil {
			t.Fatalf("AppendAuditEvent %d: %v", i, err)
		}
	}
}

// TestMigrationIdempotence opens the store twice in sequence — the
// second Open must see every migration as already-applied and be a
// no-op rather than erroring on "relation already exists".
func TestMigrationIdempotence(t *testing.T) {
	first, err := controlplane.Open(dsn(t))
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = first.Close()

	second, err := controlplane.Open(dsn(t))
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	_ = second.Close()
}

// TestConcurrentAudit hammers AppendAuditEvent from N goroutines. The
// Postgres pool must survive the concurrency without error.
func TestConcurrentAudit(t *testing.T) {
	s := openStore(t)

	const workers = 25
	const perWorker = 20
	now := time.Now().UTC().Truncate(time.Second)

	errc := make(chan error, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				err := s.AppendAuditEvent(controlplane.AuditEvent{
					ID:      fmt.Sprintf("concurrent-%d-%d", w, i),
					At:      now.Add(time.Duration(w*perWorker+i) * time.Millisecond),
					Tool:    "t",
					Action:  "tools/call",
					Outcome: "success",
				})
				if err != nil {
					errc <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errc)
	for e := range errc {
		t.Fatalf("concurrent append failed: %v", e)
	}
}
