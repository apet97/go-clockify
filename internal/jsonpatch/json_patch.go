// Package jsonpatch implements RFC 6902 JSON Patch: a sequence of add,
// remove, replace operations that transforms one JSON document into another.
// Used as an alternative delta format for MCP resource notifications.
package jsonpatch

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Operation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

func Diff(prev, curr []byte) ([]byte, error) {
	var prevVal, currVal any
	if err := json.Unmarshal(prev, &prevVal); err != nil {
		return nil, fmt.Errorf("jsonpatch: unmarshal prev: %w", err)
	}
	if err := json.Unmarshal(curr, &currVal); err != nil {
		return nil, fmt.Errorf("jsonpatch: unmarshal curr: %w", err)
	}
	ops := diffValues("", prevVal, currVal)
	return json.Marshal(ops)
}

func Apply(doc, patch []byte) ([]byte, error) {
	var val any
	if err := json.Unmarshal(doc, &val); err != nil {
		return nil, fmt.Errorf("jsonpatch: unmarshal doc: %w", err)
	}
	var ops []Operation
	if err := json.Unmarshal(patch, &ops); err != nil {
		return nil, fmt.Errorf("jsonpatch: unmarshal patch: %w", err)
	}
	var err error
	for _, op := range ops {
		val, err = applyOp(val, op)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(val)
}

func diffValues(path string, prev, curr any) []Operation {
	prevObj, prevIsObj := prev.(map[string]any)
	currObj, currIsObj := curr.(map[string]any)

	if prevIsObj && currIsObj {
		return diffObjects(path, prevObj, currObj)
	}

	prevArr, prevIsArr := prev.([]any)
	currArr, currIsArr := curr.([]any)
	if prevIsArr && currIsArr {
		return diffArrays(path, prevArr, currArr)
	}

	if !jsonEqual(prev, curr) {
		return []Operation{{Op: "replace", Path: path, Value: curr}}
	}
	return nil
}

func diffObjects(path string, prev, curr map[string]any) []Operation {
	var ops []Operation
	for k, cv := range curr {
		childPath := path + "/" + escapePointer(k)
		pv, exists := prev[k]
		if !exists {
			ops = append(ops, Operation{Op: "add", Path: childPath, Value: cv})
			continue
		}
		ops = append(ops, diffValues(childPath, pv, cv)...)
	}
	for k := range prev {
		if _, exists := curr[k]; !exists {
			ops = append(ops, Operation{Op: "remove", Path: path + "/" + escapePointer(k)})
		}
	}
	return ops
}

func diffArrays(path string, prev, curr []any) []Operation {
	var ops []Operation
	minLen := len(prev)
	if len(curr) < minLen {
		minLen = len(curr)
	}
	for i := 0; i < minLen; i++ {
		childPath := path + "/" + strconv.Itoa(i)
		ops = append(ops, diffValues(childPath, prev[i], curr[i])...)
	}
	for i := minLen; i < len(curr); i++ {
		ops = append(ops, Operation{Op: "add", Path: path + "/-", Value: curr[i]})
	}
	for i := len(prev) - 1; i >= minLen; i-- {
		ops = append(ops, Operation{Op: "remove", Path: path + "/" + strconv.Itoa(i)})
	}
	return ops
}

func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

func unescapePointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

func jsonEqual(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

func applyOp(doc any, op Operation) (any, error) {
	if op.Path == "" {
		switch op.Op {
		case "replace":
			return op.Value, nil
		default:
			return nil, fmt.Errorf("jsonpatch: op %q on root is not supported", op.Op)
		}
	}

	parts := splitPointer(op.Path)
	return applyAtPath(doc, parts, op)
}

func splitPointer(path string) []string {
	if path == "" {
		return nil
	}
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.Split(trimmed, "/")
	for i, p := range parts {
		parts[i] = unescapePointer(p)
	}
	return parts
}

func applyAtPath(doc any, parts []string, op Operation) (any, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("jsonpatch: empty path parts")
	}

	key := parts[0]

	if len(parts) == 1 {
		switch obj := doc.(type) {
		case map[string]any:
			return applyToObject(obj, key, op)
		case []any:
			return applyToArray(obj, key, op)
		default:
			return nil, fmt.Errorf("jsonpatch: cannot index into %T", doc)
		}
	}

	switch obj := doc.(type) {
	case map[string]any:
		child, exists := obj[key]
		if !exists {
			return nil, fmt.Errorf("jsonpatch: path %q not found", key)
		}
		newChild, err := applyAtPath(child, parts[1:], op)
		if err != nil {
			return nil, err
		}
		obj[key] = newChild
		return obj, nil
	case []any:
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(obj) {
			return nil, fmt.Errorf("jsonpatch: invalid array index %q", key)
		}
		newChild, err := applyAtPath(obj[idx], parts[1:], op)
		if err != nil {
			return nil, err
		}
		obj[idx] = newChild
		return obj, nil
	default:
		return nil, fmt.Errorf("jsonpatch: cannot traverse %T", doc)
	}
}

func applyToObject(obj map[string]any, key string, op Operation) (any, error) {
	switch op.Op {
	case "add", "replace":
		obj[key] = op.Value
		return obj, nil
	case "remove":
		delete(obj, key)
		return obj, nil
	default:
		return nil, fmt.Errorf("jsonpatch: unsupported op %q", op.Op)
	}
}

func applyToArray(arr []any, key string, op Operation) (any, error) {
	if key == "-" && op.Op == "add" {
		return append(arr, op.Value), nil
	}
	idx, err := strconv.Atoi(key)
	if err != nil {
		return nil, fmt.Errorf("jsonpatch: invalid array index %q", key)
	}
	switch op.Op {
	case "add":
		if idx < 0 || idx > len(arr) {
			return nil, fmt.Errorf("jsonpatch: index %d out of range", idx)
		}
		result := make([]any, 0, len(arr)+1)
		result = append(result, arr[:idx]...)
		result = append(result, op.Value)
		result = append(result, arr[idx:]...)
		return result, nil
	case "replace":
		if idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("jsonpatch: index %d out of range", idx)
		}
		arr[idx] = op.Value
		return arr, nil
	case "remove":
		if idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("jsonpatch: index %d out of range", idx)
		}
		return append(arr[:idx], arr[idx+1:]...), nil
	default:
		return nil, fmt.Errorf("jsonpatch: unsupported op %q", op.Op)
	}
}
