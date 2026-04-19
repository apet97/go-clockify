package harness

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaitForHTTP200 polls the given URL every 20ms until it returns HTTP 200
// or until ctx expires (or a 5-second deadline if ctx has none). Used by
// adapters to avoid racing Initialize against the server goroutine.
func WaitForHTTP200(ctx context.Context, url string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(5 * time.Second)
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if lastErr != nil {
		return fmt.Errorf("timeout waiting for %s: %w", url, lastErr)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}
