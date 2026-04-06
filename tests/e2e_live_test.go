package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/tools"
)

func setupTestEnv(t *testing.T) *tools.Service {
	// Require Env Vars for live test
	if os.Getenv("CLOCKIFY_API_KEY") == "" {
		t.Skip("Skipping live e2e tests since CLOCKIFY_API_KEY is not set")
	}
	os.Setenv("CLOCKIFY_DRY_RUN", "off") // Allow real mutations

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, cfg.RequestTimeout, cfg.MaxRetries)
	service := tools.New(client, cfg.WorkspaceID)

	return service
}

func invokeTool(ctx context.Context, service *tools.Service, name string, args map[string]any) (tools.ResultEnvelope, error) {
	for _, tool := range service.Registry() {
		if tool.Tool.Name == name {
			resAny, err := tool.Handler(ctx, args)
			if err != nil {
				return tools.ResultEnvelope{}, err
			}
			resEnv, ok := resAny.(tools.ResultEnvelope)
			if !ok {
				return tools.ResultEnvelope{}, fmt.Errorf("unexpected return type not ResultEnvelope")
			}
			return resEnv, nil
		}
	}
	return tools.ResultEnvelope{}, fmt.Errorf("tool %s not found", name)
}

func unmarshalData[T any](data any, out *T) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func TestE2EReadOnly(t *testing.T) {
	svc := setupTestEnv(t)
	ctx := context.Background()

	// 1. whoami
	resEnv, err := invokeTool(ctx, svc, "clockify_whoami", nil)
	if err != nil {
		t.Fatalf("whoami failed: %v", err)
	}
	t.Logf("whoami success")

	// 2. get_workspace
	resEnv, err = invokeTool(ctx, svc, "clockify_get_workspace", nil)
	if err != nil {
		t.Fatalf("get_workspace failed: %v", err)
	}
	var ws map[string]any
	if err := unmarshalData(resEnv.Data, &ws); err == nil {
		t.Logf("get_workspace success: %v", ws["id"])
	}

	// 3. list_projects
	resEnv, err = invokeTool(ctx, svc, "clockify_list_projects", nil)
	if err != nil {
		t.Fatalf("list_projects failed: %v", err)
	}
	var projects []clockify.Project
	if err := unmarshalData(resEnv.Data, &projects); err != nil {
		t.Fatalf("Unexpected projects format")
	}
	t.Logf("list_projects success: found %d projects", len(projects))
}

func TestE2EMutating(t *testing.T) {
	svc := setupTestEnv(t)
	ctx := context.Background()
	tagPrefix := fmt.Sprintf("AG_TEST_%d", time.Now().Unix())

	// 1. Create a client
	cResEnv, err := invokeTool(ctx, svc, "clockify_create_client", map[string]any{"name": tagPrefix + "_client"})
	if err != nil {
		t.Fatalf("create_client failed: %v", err)
	}
	var client clockify.ClientEntity
	if err := unmarshalData(cResEnv.Data, &client); err != nil {
		t.Fatalf("Unexpected client return format")
	}
	t.Logf("created client: %s", client.ID)

	// 2. Create a project
	pResEnv, err := invokeTool(ctx, svc, "clockify_create_project", map[string]any{
		"name": tagPrefix + "_project",
		"client": client.ID,
	})
	if err != nil {
		t.Fatalf("create_project failed: %v", err)
	}
	var project clockify.Project
	if err := unmarshalData(pResEnv.Data, &project); err != nil {
		t.Fatalf("Unexpected project return format")
	}
	t.Logf("created project: %s", project.ID)

	// 3. Start a timer
	startResEnv, err := invokeTool(ctx, svc, "clockify_start_timer", map[string]any{
		"project_id": project.ID,
		"description": "E2E testing timer",
	})
	if err != nil {
		t.Fatalf("start_timer failed: %v", err)
	}
	t.Logf("started timer: %v", startResEnv.Data)

	// 4. Stop the timer
	time.Sleep(1 * time.Second) // wait slightly
	stopResEnv, err := invokeTool(ctx, svc, "clockify_stop_timer", map[string]any{"dry_run": false})
	if err != nil {
		t.Fatalf("stop_timer failed: %v", err)
	}
	var entry clockify.TimeEntry
	if err := unmarshalData(stopResEnv.Data, &entry); err != nil {
		t.Fatalf("Unexpected stop timer return format")
	}
	t.Logf("stopped timer entry: %s", entry.ID)

	// 5. Cleanup time entry
	_, err = invokeTool(ctx, svc, "clockify_delete_entry", map[string]any{"entry_id": entry.ID, "dry_run": false})
	if err != nil {
		t.Fatalf("delete_entry failed: %v", err)
	}
	t.Logf("deleted entry: %s", entry.ID)

	t.Logf("Test data left over: Client: %s, Project: %s", client.ID, project.ID)
}

func TestE2EErrors(t *testing.T) {
	svc := setupTestEnv(t)
	ctx := context.Background()

	// Invalid entry ID
	_, err := invokeTool(ctx, svc, "clockify_get_entry", map[string]any{"entry_id": "invalid_12345"})
	if err == nil {
		t.Fatalf("Expected error for invalid entry_id but got none")
	}
	t.Logf("Correctly got error for invalid entry_id: %v", err)

	// Missing args
	_, err = invokeTool(ctx, svc, "clockify_create_project", map[string]any{})
	if err == nil {
		t.Fatalf("Expected error for missing required args in create_project")
	}
	t.Logf("Correctly got error for missing args: %v", err)
}
