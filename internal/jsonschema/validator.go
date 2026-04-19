// Package jsonschema is a tiny stdlib-only JSON-schema validator scoped to
// the keyword subset that the Clockify MCP server's Tier 1 + Tier 2 tool
// input schemas actually use. It is intentionally narrow:
//
// Supported keywords:
//   - type: object, string, integer, number, boolean, array
//   - required (array of strings on objects)
//   - additionalProperties: false (true is a no-op)
//   - properties (recursive)
//   - items (array element schema)
//   - minimum / maximum (integer and number)
//   - minLength / maxLength (string)
//   - pattern (regexp; anchored via ^...$ if not already)
//   - format: date / date-time (lenient time.Parse)
//   - enum (array of any; exact JSON-equal match)
//
// Not supported (deliberately): $ref, $defs, allOf/anyOf/oneOf, not,
// conditionals (if/then/else), dependentSchemas, const,
// exclusiveMinimum/exclusiveMaximum, multipleOf, propertyNames,
// patternProperties. None of those keywords appear in the Tier 1 or
// Tier 2 fleet and adding them would pull complexity with no caller.
//
// Values are validated against the JSON-shaped tree the caller passes in,
// which in the server is always a map[string]any produced by
// json.Unmarshal. Integer-vs-number distinction follows Go's native type
// system: json.Unmarshal decodes every numeric literal as float64, so
// `type: integer` accepts any float64 whose fractional part is zero.
//
// The validator returns *ValidationError on failure. The caller is
// expected to wrap that inside a protocol-specific error type (e.g.
// mcp.InvalidParamsError) and emit the wire response.
package jsonschema

import (
	"fmt"
	"reflect"
	"regexp"
	"sync"
	"time"
)

// ValidationError reports a single validation failure with its JSON
// Pointer path (RFC 6901). Pointer is empty ("") when the root value
// itself is the offender.
type ValidationError struct {
	Pointer string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Pointer == "" {
		return "invalid params: " + e.Message
	}
	return fmt.Sprintf("invalid params at %s: %s", e.Pointer, e.Message)
}

// patternCache memoizes compiled regexps for reuse across calls on the
// same schema literal. Keyed by the exact pattern string; the anchor
// wrapping rule is deterministic so a cache hit is always safe.
var patternCache sync.Map // map[string]*regexp.Regexp

// Validate reports whether value conforms to schema. A nil schema is
// treated as "no constraints" and always succeeds. A nil value against
// a schema with type: object is rejected as a type error so the caller
// sees the failing path rather than a segfault.
func Validate(schema map[string]any, value any) error {
	if schema == nil {
		return nil
	}
	return validate(schema, value, "")
}

func validate(schema map[string]any, value any, ptr string) error {
	if schema == nil {
		return nil
	}

	// enum is checked before type because an enum value may legitimately
	// be of a type the schema doesn't restrict elsewhere.
	if enum, ok := schema["enum"]; ok {
		if err := checkEnum(enum, value, ptr); err != nil {
			return err
		}
	}

	typ, _ := schema["type"].(string)
	switch typ {
	case "":
		// Untyped schema — validate subkeywords that don't depend on type.
	case "object":
		return validateObject(schema, value, ptr)
	case "array":
		return validateArray(schema, value, ptr)
	case "string":
		return validateString(schema, value, ptr)
	case "integer":
		return validateInteger(schema, value, ptr)
	case "number":
		return validateNumber(schema, value, ptr)
	case "boolean":
		if _, ok := value.(bool); !ok {
			return typeError(ptr, "boolean", value)
		}
	default:
		// Unknown type keyword — treat as no-op to stay forward-compatible
		// with tightenings that introduce new type strings we don't know
		// about yet.
	}
	return nil
}

func validateObject(schema map[string]any, value any, ptr string) error {
	obj, ok := value.(map[string]any)
	if !ok {
		return typeError(ptr, "object", value)
	}

	// required
	if req, ok := schema["required"]; ok {
		names, err := toStringSlice(req)
		if err == nil {
			for _, name := range names {
				if _, present := obj[name]; !present {
					return &ValidationError{
						Pointer: joinPtr(ptr, name),
						Message: "missing required property",
					}
				}
			}
		}
	}

	props, _ := schema["properties"].(map[string]any)

	// additionalProperties: false rejects unknown keys. Explicit `true`
	// or a missing key both allow extras.
	if ap, set := schema["additionalProperties"]; set {
		if allow, ok := ap.(bool); ok && !allow {
			for k := range obj {
				if props == nil {
					return &ValidationError{
						Pointer: joinPtr(ptr, k),
						Message: "unknown property",
					}
				}
				if _, known := props[k]; !known {
					return &ValidationError{
						Pointer: joinPtr(ptr, k),
						Message: "unknown property",
					}
				}
			}
		}
	}

	// properties — validate each present key against its subschema.
	// ranging a nil map is a no-op, so no explicit nil guard needed.
	for name, raw := range props {
		sub, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if val, present := obj[name]; present {
			if err := validate(sub, val, joinPtr(ptr, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateArray(schema map[string]any, value any, ptr string) error {
	arr, ok := value.([]any)
	if !ok {
		return typeError(ptr, "array", value)
	}
	items, _ := schema["items"].(map[string]any)
	if items == nil {
		return nil
	}
	for i, elem := range arr {
		if err := validate(items, elem, fmt.Sprintf("%s/%d", ptr, i)); err != nil {
			return err
		}
	}
	return nil
}

func validateString(schema map[string]any, value any, ptr string) error {
	s, ok := value.(string)
	if !ok {
		return typeError(ptr, "string", value)
	}
	if raw, ok := schema["minLength"]; ok {
		if n, ok := toInt(raw); ok && len(s) < n {
			return &ValidationError{Pointer: ptr, Message: fmt.Sprintf("string shorter than minLength %d", n)}
		}
	}
	if raw, ok := schema["maxLength"]; ok {
		if n, ok := toInt(raw); ok && len(s) > n {
			return &ValidationError{Pointer: ptr, Message: fmt.Sprintf("string longer than maxLength %d", n)}
		}
	}
	if raw, ok := schema["pattern"].(string); ok && raw != "" {
		re, err := compilePattern(raw)
		if err != nil {
			return &ValidationError{Pointer: ptr, Message: "schema pattern is not a valid regexp"}
		}
		if !re.MatchString(s) {
			return &ValidationError{Pointer: ptr, Message: fmt.Sprintf("string does not match pattern %q", raw)}
		}
	}
	if raw, ok := schema["format"].(string); ok && raw != "" {
		if err := checkFormat(raw, s, ptr); err != nil {
			return err
		}
	}
	return nil
}

func validateInteger(schema map[string]any, value any, ptr string) error {
	n, ok := toFloat(value)
	if !ok {
		return typeError(ptr, "integer", value)
	}
	if n != float64(int64(n)) {
		return typeError(ptr, "integer", value)
	}
	return checkNumericBounds(schema, n, ptr)
}

func validateNumber(schema map[string]any, value any, ptr string) error {
	n, ok := toFloat(value)
	if !ok {
		return typeError(ptr, "number", value)
	}
	return checkNumericBounds(schema, n, ptr)
}

func checkNumericBounds(schema map[string]any, n float64, ptr string) error {
	if raw, ok := schema["minimum"]; ok {
		if f, ok := toFloat(raw); ok && n < f {
			return &ValidationError{Pointer: ptr, Message: fmt.Sprintf("value %v below minimum %v", n, f)}
		}
	}
	if raw, ok := schema["maximum"]; ok {
		if f, ok := toFloat(raw); ok && n > f {
			return &ValidationError{Pointer: ptr, Message: fmt.Sprintf("value %v above maximum %v", n, f)}
		}
	}
	return nil
}

func checkFormat(format, s, ptr string) error {
	switch format {
	case "date-time":
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return &ValidationError{Pointer: ptr, Message: "string is not a valid RFC3339 date-time"}
		}
	case "date":
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return &ValidationError{Pointer: ptr, Message: "string is not a valid YYYY-MM-DD date"}
		}
	default:
		// Unknown format — spec says this is annotation-only, so no-op.
	}
	return nil
}

func checkEnum(enum, value any, ptr string) error {
	options, ok := enum.([]any)
	if !ok {
		// tool author might have authored []string — normalize.
		if ss, err := toStringSlice(enum); err == nil {
			for _, s := range ss {
				if reflect.DeepEqual(s, value) {
					return nil
				}
			}
			return &ValidationError{Pointer: ptr, Message: "value not in enum"}
		}
		return nil
	}
	for _, opt := range options {
		if enumEqual(opt, value) {
			return nil
		}
	}
	return &ValidationError{Pointer: ptr, Message: "value not in enum"}
}

// enumEqual compares an enum option to a candidate value. Numeric types
// are compared after float64 coercion so `1` (int) equals `1.0`.
func enumEqual(a, b any) bool {
	if af, aok := toFloat(a); aok {
		if bf, bok := toFloat(b); bok {
			return af == bf
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}

// toStringSlice accepts []string, []any (of strings), or anything else —
// it returns an empty slice and an error for the anything-else case so
// callers can decide to fall back.
func toStringSlice(v any) ([]string, error) {
	switch x := v.(type) {
	case []string:
		return x, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("not a string slice")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("not a string slice")
	}
}

// toFloat coerces any numeric Go value (including the untyped integers
// tool authors use in schema literals) to float64.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func toInt(v any) (int, bool) {
	f, ok := toFloat(v)
	if !ok {
		return 0, false
	}
	if f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

func typeError(ptr, want string, got any) *ValidationError {
	return &ValidationError{
		Pointer: ptr,
		Message: fmt.Sprintf("expected %s, got %s", want, typeName(got)),
	}
}

func typeName(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return reflect.TypeOf(v).String()
	}
}

// joinPtr appends a property name to a JSON Pointer, escaping `~` and
// `/` per RFC 6901.
func joinPtr(prefix, name string) string {
	escaped := escapePtr(name)
	return prefix + "/" + escaped
}

func escapePtr(s string) string {
	// RFC 6901: ~ → ~0, / → ~1 (order matters).
	// Fast path: check if escaping is needed before allocating.
	needsEscaping := false
	for i := 0; i < len(s); i++ {
		if s[i] == '~' || s[i] == '/' {
			needsEscaping = true
			break
		}
	}
	if !needsEscaping {
		return s
	}

	out := make([]byte, 0, len(s)+2)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}

// compilePattern returns a cached compiled regexp. If the source pattern
// is not already anchored with ^ and $, the cached entry wraps it so
// pattern matching behaves like JSON-schema's implicit anchoring for
// format constraints. (Raw Go regexp is substring-match by default.)
func compilePattern(p string) (*regexp.Regexp, error) {
	if raw, ok := patternCache.Load(p); ok {
		if re, ok := raw.(*regexp.Regexp); ok {
			return re, nil
		}
	}
	src := p
	if len(src) == 0 || src[0] != '^' {
		src = "^" + src
	}
	if src[len(src)-1] != '$' {
		src = src + "$"
	}
	re, err := regexp.Compile(src)
	if err != nil {
		return nil, err
	}
	patternCache.Store(p, re)
	return re, nil
}
