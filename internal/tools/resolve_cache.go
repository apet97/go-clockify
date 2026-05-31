package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/resolve"
)

const (
	resolveCacheTTL            = time.Minute
	resolveCacheSweepThreshold = 1024
)

type resolveKey struct {
	kind  string
	scope string
	ref   string
}

type resolveEntry struct {
	id        string
	expiresAt time.Time
}

func (s *Service) resolveProjectID(ctx context.Context, workspaceID, ref string) (string, error) {
	return s.resolveCached("project", workspaceID, ref, func() (string, error) {
		return resolve.ProjectID(ctx, s.Client, workspaceID, ref)
	})
}

func (s *Service) resolveClientID(ctx context.Context, workspaceID, ref string) (string, error) {
	return s.resolveCached("client", workspaceID, ref, func() (string, error) {
		return resolve.ClientID(ctx, s.Client, workspaceID, ref)
	})
}

func (s *Service) resolveTagID(ctx context.Context, workspaceID, ref string) (string, error) {
	return s.resolveCached("tag", workspaceID, ref, func() (string, error) {
		return resolve.TagID(ctx, s.Client, workspaceID, ref)
	})
}

func (s *Service) resolveUserID(ctx context.Context, workspaceID, ref string) (string, error) {
	return s.resolveCached("user", workspaceID, ref, func() (string, error) {
		return resolve.UserID(ctx, s.Client, workspaceID, ref)
	})
}

func (s *Service) resolveTaskID(ctx context.Context, workspaceID, projectID, ref string) (string, error) {
	scope := workspaceID + "\x00" + projectID
	return s.resolveCached("task", scope, ref, func() (string, error) {
		return resolve.TaskID(ctx, s.Client, workspaceID, projectID, ref)
	})
}

func (s *Service) resolveCached(kind, scope, ref string, resolveFn func() (string, error)) (string, error) {
	key, ok := cacheableResolveKey(kind, scope, ref)
	if !ok {
		return resolveFn()
	}
	now := time.Now()
	if id, ok := s.resolveCacheGet(key, now); ok {
		return id, nil
	}
	id, err := resolveFn()
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("%s response missing id", kind)
	}
	s.resolveCacheSet(key, id, now.Add(resolveCacheTTL))
	return id, nil
}

func cacheableResolveKey(kind, scope, ref string) (resolveKey, bool) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" || looksLikeClockifyIDForResolveCache(trimmed) {
		return resolveKey{}, false
	}
	if kind == "user" && looksLikeEmailForResolveCache(trimmed) {
		return resolveKey{}, false
	}
	return resolveKey{kind: kind, scope: scope, ref: strings.ToLower(trimmed)}, true
}

func (s *Service) resolveCacheGet(key resolveKey, now time.Time) (string, bool) {
	if s == nil {
		return "", false
	}
	return s.resolver.get(key, now)
}

func (s *Service) resolveCacheSet(key resolveKey, id string, expiresAt time.Time) {
	if s == nil {
		return
	}
	s.resolver.set(key, id, expiresAt)
}

func looksLikeEmailForResolveCache(s string) bool {
	at := strings.IndexByte(s, '@')
	if at < 1 || at >= len(s)-1 {
		return false
	}
	dot := strings.LastIndexByte(s[at:], '.')
	return dot > 1
}

func looksLikeClockifyIDForResolveCache(value string) bool {
	if len(value) != 24 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}
