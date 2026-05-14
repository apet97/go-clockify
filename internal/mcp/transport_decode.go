//go:build legacy_platform

package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func decodeSingleRequest(r io.Reader) (Request, bool, error) {
	var req Request
	dec := json.NewDecoder(r)
	if err := dec.Decode(&req); err != nil {
		if !isMaxBytesError(err) {
			if _, drainErr := io.Copy(io.Discard, r); isMaxBytesError(drainErr) {
				return Request{}, true, drainErr
			}
		}
		return Request{}, isMaxBytesError(err), err
	}

	var extra any
	if err := dec.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return req, false, nil
		}
		return Request{}, isMaxBytesError(err), err
	}
	return Request{}, false, errors.New("multiple JSON values in request body")
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
