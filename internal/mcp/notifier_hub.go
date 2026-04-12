package mcp

import (
	"log/slog"
	"sync"
)

type notifierHub struct {
	mu        sync.RWMutex
	notifiers map[uint64]Notifier
	nextID    uint64
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
		delete(h.notifiers, id)
		h.mu.Unlock()
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
