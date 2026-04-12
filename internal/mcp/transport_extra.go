package mcp

import "net/http"

// ExtraHandler is a neutral pattern+handler pair that transports mount on
// their internal mux before ListenAndServe. The field exists so debug or
// observability handlers owned by cmd/ can be plugged onto the same listener
// as /mcp without forcing internal/mcp to import anything outside the stdlib.
//
// Intentionally minimal: no middleware hooks, no auth toggles, no priority
// field. The only consumer today is the -tags=pprof build in
// cmd/clockify-mcp/, which mounts /debug/pprof/* against
// http.DefaultServeMux. Adding more knobs here means internal/mcp starts
// caring about debug concerns it shouldn't care about.
type ExtraHandler struct {
	Pattern string
	Handler http.Handler
}

// mountExtras wires each extra onto the supplied mux, wrapping it in
// observeHTTPH so the same metrics + panic-recovery middleware that covers
// real endpoints also covers debug surfaces. nil or empty slice is a no-op,
// which is the default path for every production build.
func mountExtras(mux *http.ServeMux, extras []ExtraHandler) {
	for _, e := range extras {
		if e.Pattern == "" || e.Handler == nil {
			continue
		}
		mux.Handle(e.Pattern, observeHTTPH(e.Pattern, e.Handler))
	}
}
