package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/resolve"
)

// OneUserConfig is the complete runtime configuration for the
// one-user/full-access stdio product path.
type OneUserConfig struct {
	APIKey      string
	WorkspaceID string
	Timezone    string
	BaseURL     string
	LogLevel    string
}

// LoadOneUser reads the intentionally tiny environment surface used by the
// one-user stdio runtime. It ignores the broader platform-era configuration
// matrix by design.
func LoadOneUser() (OneUserConfig, error) {
	cfg := OneUserConfig{
		APIKey:      strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY")),
		WorkspaceID: strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID")),
		Timezone:    strings.TrimSpace(os.Getenv("CLOCKIFY_TIMEZONE")),
		BaseURL:     strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL")),
		LogLevel:    strings.TrimSpace(os.Getenv("MCP_LOG_LEVEL")),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.APIKey == "" {
		return OneUserConfig{}, fmt.Errorf("CLOCKIFY_API_KEY is required")
	}
	if cfg.WorkspaceID == "" {
		return OneUserConfig{}, fmt.Errorf("CLOCKIFY_WORKSPACE_ID is required")
	}
	if err := resolve.ValidateID(cfg.WorkspaceID, "CLOCKIFY_WORKSPACE_ID"); err != nil {
		return OneUserConfig{}, err
	}
	if cfg.Timezone != "" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			return OneUserConfig{}, fmt.Errorf("invalid CLOCKIFY_TIMEZONE: %w", err)
		}
	}
	if err := validateOneUserBaseURL(cfg.BaseURL); err != nil {
		return OneUserConfig{}, err
	}
	return cfg, nil
}

func validateOneUserBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: %w", err)
	}
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: HTTPS is required unless the host is loopback")
	}
	if u.Host == "" {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: host is required")
	}
	return nil
}
