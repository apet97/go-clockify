package mcp

import (
	"log/slog"
	"sync"
)

type notifierHub struct {
	mu        sync.RWMutex
	notifiers map[uint64]Notifier
	nextID    uint64
	// onRemove fires after a notifier is removed from the hub (stream
	// close, SetNotifier eviction). Server uses this to drop any
	// resources/subscribe state the notifier accumulated, so the
	// HasResourceSubscription shortcut and per-URI fan-out stop
	// counting disconnected streams.
	onRemove func(Notifier)
}

func (h *notifierHub) add(n Notifier) func() {
	h.mu.Lock()
	if h.notifiers == nil {
		h.notifiers = make(map[uint64]Notifier)
	}
	id := h.nextID
	h.nextID++
	h.notifiers[id] = n
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		_, present := h.notifiers[id]
		delete(h.notifiers, id)
		cb := h.onRemove
		h.mu.Unlock()
		if present && cb != nil {
			cb(n)
		}
	}
}

func (h *notifierHub) notify(method string, params any) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var firstErr error
	for _, n := range h.notifiers {
		if err := n.Notify(method, params); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Warn("notifier_fan_out_error",
				"method", method,
				"error", err.Error(),
			)
		}
	}
	return firstErr
}

func (h *notifierHub) len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.notifiers)
}
