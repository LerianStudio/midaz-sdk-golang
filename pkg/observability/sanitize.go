package observability

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

const (
	redactedValue = "[REDACTED]"

	// sanitizeMaxScanBytes caps the slice we scan for sensitive tokens. The
	// regex passes are O(n) but with non-trivial constants; an unbounded
	// scan turns multi-megabyte response bodies into a denial-of-service
	// vector for log redaction. Truncating the scan window keeps redaction
	// bounded under pathological input; the unscanned tail is never returned
	// verbatim.
	//
	// 64 KiB covers typical HTTP error bodies (4xx/5xx JSON responses rarely
	// exceed a few KiB) and the larger upstream payloads we have seen in
	// practice, while keeping a single ReplaceAllString pass well under a
	// millisecond on commodity hardware. Callers that intentionally pass
	// larger blobs through here should redact at the source instead of
	// relying on this best-effort post-hoc scrub.
	sanitizeMaxScanBytes = 64 * 1024

	// sanitizeMaxDepth bounds recursive structured-field redaction so cyclic or
	// pathological values cannot make logging unbounded.
	sanitizeMaxDepth = 8
)

// sensitiveAssignmentPattern matches "<sensitive-key>=<value>" or
// "<sensitive-key>:<value>" where the value may be bare, single-quoted,
// or double-quoted. Aligned with pkg/errors so quoted credential forms
// land in both redaction layers consistently — the prior bare-only
// pattern (`[^\s&;,]+`) silently passed over `password="hunter2"` in
// observability logs while pkg/errors caught it.
//
// The trailing `(?:\.[\w.-]+)?` on the keyword group allows compound
// suffixes like `metadata.user.email=…` to be redacted as a single
// match. The PII identifier list (document, legal_document, external_id,
// banking_details_*, related_party_document, regulatory_fields_*) is
// kept in lockstep with pkg/errors.sensitiveKeyValuePattern so the same
// payload renders identically through both redaction layers.
var (
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(["']?)(access[_.-]?token|api[_.-]?key|apikey|auth[_.-]?token|client[_.-]?secret|id[_.-]?token|password|secret|token|refresh[_.-]?token|x[_.-]?api[_.-]?key|x-idempotency|idempotency-key|document|legal_document|external_id|banking_details_account|banking_details_iban|metadata|related_party_document|regulatory_fields_participant_document)(?:\.[\w.-]+)?(["']?)(\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s&;,}]+)`)
	authTokenPattern           = regexp.MustCompile(`(?i)((?:bearer|basic)\s+)[A-Za-z0-9._\-+/=]+`)
)

type sanitizeVisit struct {
	kind reflect.Kind
	ptr  uintptr
}

// sanitizeSensitiveString redacts secrets from value. It is hot-pathed by the
// fast preflight checks below — the regex engine is only invoked when the
// input plausibly contains a sensitive marker.
//
// Fast path:
//   - empty string → return as-is.
//   - no '=' AND no ':' AND no case-insensitive auth scheme → return as-is.
//
// Slow path: scan up to sanitizeMaxScanBytes for the assignment + auth-token
// patterns. Inputs beyond the scan window return the redacted prefix plus a
// truncation sentinel so secrets cannot leak from the unscanned tail.
func sanitizeSensitiveString(value string) string {
	if value == "" {
		return ""
	}

	if !mayContainSensitiveToken(value) {
		return value
	}

	scan := value
	truncated := false
	if len(scan) > sanitizeMaxScanBytes {
		scan = value[:sanitizeMaxScanBytes]
		truncated = true
	}

	sanitized := sensitiveAssignmentPattern.ReplaceAllString(scan, `${1}${2}${3}${4}`+redactedValue)
	sanitized = authTokenPattern.ReplaceAllString(sanitized, `${1}`+redactedValue)

	if !truncated {
		return sanitized
	}

	return sanitized + " [truncated]"
}

func sanitizeLogFieldValue(key string, value any) any {
	// Hot-path fast lane: when the value is a plain primitive there is
	// no container to track, so we skip the seen-map allocation. The
	// rare value that turns out to be a container reflects through to
	// [sanitizeAny] with a freshly-allocated map below.
	if !isSensitiveLogFieldKey(key) {
		if v, ok := fastSanitizePrimitive(value); ok {
			return v
		}
	}

	return sanitizeLogFieldValueWithState(key, value, 0, nil)
}

// fastSanitizePrimitive returns (sanitized, true) when value is one of
// the trivially-safe primitive types that does not need recursion or a
// visit-set. The vast majority of structured-log values are bool / int /
// float / nil / short string — this fast lane skips the reflect path and
// the seen-map allocation entirely.
//
// Numeric types are handled in a separate helper to keep this switch
// under the project's cyclomatic-complexity gate; the split is cosmetic
// (the inlined version is identical in behaviour).
func fastSanitizePrimitive(value any) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, true
	case bool:
		return v, true
	case string:
		return sanitizeSensitiveString(v), true
	}

	return fastSanitizeNumeric(value)
}

func fastSanitizeNumeric(value any) (any, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return v, true
	case int16:
		return v, true
	case int32:
		return v, true
	case int64:
		return v, true
	case uint:
		return v, true
	case uint8:
		return v, true
	case uint16:
		return v, true
	case uint32:
		return v, true
	case uint64:
		return v, true
	case uintptr:
		return v, true
	case float32:
		return v, true
	case float64:
		return v, true
	}

	return nil, false
}

func sanitizeLogFieldValueWithState(key string, value any, depth int, seen map[sanitizeVisit]struct{}) any {
	if isSensitiveLogFieldKey(key) {
		return redactedValue
	}

	return sanitizeAny(value, depth, seen)
}

// ensureSeen lazily allocates the visit-set the first time the recursive
// path actually encounters a container. Most log values are scalar and
// never trip this branch, so the map alloc happens only when needed.
func ensureSeen(seen map[sanitizeVisit]struct{}) map[sanitizeVisit]struct{} {
	if seen != nil {
		return seen
	}

	return make(map[sanitizeVisit]struct{}, 4)
}

func sanitizeAny(value any, depth int, seen map[sanitizeVisit]struct{}) any {
	if value == nil {
		return nil
	}
	if isTypedNil(value) {
		return nil
	}

	if depth >= sanitizeMaxDepth {
		return redactedValue
	}

	switch v := value.(type) {
	case string:
		return sanitizeSensitiveString(v)
	case []string:
		return sanitizeStringSlice(v)
	case []any:
		return sanitizeAnySlice(v, depth, seen)
	case map[string]string:
		return sanitizeStringMap(v)
	case map[string]any:
		return sanitizeAnyMap(v, depth, seen)
	case error:
		return sanitizeSensitiveString(v.Error())
	case fmt.Stringer:
		return sanitizeSensitiveString(v.String())
	}

	rv := reflect.ValueOf(value)
	return sanitizeReflectValue(rv, depth, ensureSeen(seen))
}

// sanitizeStringSlice redacts every element of a []string by passing it
// through [sanitizeSensitiveString]. Returns a fresh slice; the input
// slice is never mutated.
func sanitizeStringSlice(values []string) []string {
	out := make([]string, len(values))
	for i, item := range values {
		out[i] = sanitizeSensitiveString(item)
	}

	return out
}

// sanitizeStringMap redacts every value of a map[string]string. Keys
// matching [isSensitiveLogFieldKey] are short-circuited to redactedValue;
// the rest go through [sanitizeSensitiveString].
func sanitizeStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for mapKey, mapValue := range in {
		if isSensitiveLogFieldKey(mapKey) {
			out[mapKey] = redactedValue
			continue
		}

		out[mapKey] = sanitizeSensitiveString(mapValue)
	}

	return out
}

// sanitizeAnySlice handles []any with cycle detection via the visit set.
func sanitizeAnySlice(values []any, depth int, seen map[sanitizeVisit]struct{}) any {
	seen = ensureSeen(seen)
	tracked, ok := trackReference(reflect.ValueOf(values), seen)
	if !ok {
		return redactedValue
	}
	if tracked.ptr != 0 {
		defer delete(seen, tracked)
	}

	out := make([]any, len(values))
	for i, item := range values {
		out[i] = sanitizeAny(item, depth+1, seen)
	}

	return out
}

// sanitizeAnyMap handles map[string]any with cycle detection and per-key
// sensitivity classification.
func sanitizeAnyMap(in map[string]any, depth int, seen map[sanitizeVisit]struct{}) any {
	seen = ensureSeen(seen)
	tracked, ok := trackReference(reflect.ValueOf(in), seen)
	if !ok {
		return redactedValue
	}
	if tracked.ptr != 0 {
		defer delete(seen, tracked)
	}

	out := make(map[string]any, len(in))
	for mapKey, mapValue := range in {
		out[mapKey] = sanitizeLogFieldValueWithState(mapKey, mapValue, depth+1, seen)
	}

	return out
}

func sanitizeReflectValue(value reflect.Value, depth int, seen map[sanitizeVisit]struct{}) any {
	if !value.IsValid() {
		return nil
	}

	if depth >= sanitizeMaxDepth {
		return redactedValue
	}

	value, ok := unwrapReflectInterface(value)
	if !ok {
		return nil
	}

	if value.Kind() == reflect.Pointer {
		return sanitizeReflectPointer(value, depth, seen)
	}

	if tracked, ok := trackReference(value, seen); !ok {
		return redactedValue
	} else if tracked.ptr != 0 {
		defer delete(seen, tracked)
	}

	if v, ok := reflectScalar(value); ok {
		return v
	}

	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return sanitizeReflectSlice(value, depth, seen)
	case reflect.Map:
		return sanitizeReflectMap(value, depth, seen)
	case reflect.Struct:
		return sanitizeStructValue(value, depth, seen)
	default:
		if !value.CanInterface() {
			return redactedValue
		}

		return sanitizeSensitiveString(fmt.Sprint(value.Interface()))
	}
}

// reflectScalar handles the leaf primitive kinds in the reflect path
// (string/bool/numeric). Hoisted out of sanitizeReflectValue to keep the
// outer switch under the cyclomatic-complexity gate.
func reflectScalar(value reflect.Value) (any, bool) {
	switch value.Kind() {
	case reflect.String:
		return sanitizeSensitiveString(value.String()), true
	case reflect.Bool:
		return value.Bool(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint(), true
	case reflect.Float32, reflect.Float64:
		return value.Float(), true
	}

	return nil, false
}

// unwrapReflectInterface peels nested reflect.Interface wrappers until we
// hit a concrete reflect.Value. The second return is false when the chain
// terminates at an interface nil — caller treats that as "produce nil".
func unwrapReflectInterface(value reflect.Value) (reflect.Value, bool) {
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}

	return value, true
}

// sanitizeReflectPointer handles reflect.Pointer values: nil → nil, otherwise
// dereference with cycle detection and recurse.
func sanitizeReflectPointer(value reflect.Value, depth int, seen map[sanitizeVisit]struct{}) any {
	if value.IsNil() {
		return nil
	}

	tracked, ok := trackReference(value, seen)
	if !ok {
		return redactedValue
	}
	if tracked.ptr != 0 {
		defer delete(seen, tracked)
	}

	return sanitizeReflectValue(value.Elem(), depth+1, seen)
}

// sanitizeReflectSlice handles reflect.Slice and reflect.Array.
func sanitizeReflectSlice(value reflect.Value, depth int, seen map[sanitizeVisit]struct{}) []any {
	out := make([]any, value.Len())
	for i := 0; i < value.Len(); i++ {
		out[i] = sanitizeReflectValue(value.Index(i), depth+1, seen)
	}

	return out
}

// sanitizeReflectMap handles reflect.Map by Stringer-projecting keys and
// dispatching values through the sensitive-field-aware sanitizer.
func sanitizeReflectMap(value reflect.Value, depth int, seen map[sanitizeVisit]struct{}) map[string]any {
	out := make(map[string]any, value.Len())
	iter := value.MapRange()
	for iter.Next() {
		mapKey := fmt.Sprint(iter.Key().Interface())
		out[mapKey] = sanitizeLogFieldValueWithState(mapKey, iter.Value().Interface(), depth+1, seen)
	}

	return out
}

func sanitizeStructValue(value reflect.Value, depth int, seen map[sanitizeVisit]struct{}) any {
	if !value.CanInterface() {
		return redactedValue
	}

	valueType := value.Type()
	out := make(map[string]any, value.NumField())
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fieldName := field.Name
		if jsonName, ok := jsonFieldName(field.Tag.Get("json")); ok {
			if jsonName == "-" {
				continue
			}
			if jsonName != "" {
				fieldName = jsonName
			}
		}

		out[fieldName] = sanitizeLogFieldValueWithState(fieldName, value.Field(i).Interface(), depth+1, seen)
	}

	// An empty struct projection (no exported fields, or all fields tagged
	// json:"-") is not sensitive — return the empty map so callers can
	// distinguish "we redacted this" from "this carried no exported
	// content". Previously we returned redactedValue here, which made an
	// empty struct visually indistinguishable from a credential blob.
	if len(out) == 0 {
		return map[string]any{}
	}

	return out
}

func jsonFieldName(tag string) (string, bool) {
	if tag == "" {
		return "", false
	}

	name, _, _ := strings.Cut(tag, ",")
	return name, true
}

func isTypedNil(value any) bool {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func trackReference(value reflect.Value, seen map[sanitizeVisit]struct{}) (sanitizeVisit, bool) {
	for value.IsValid() && value.Kind() == reflect.Interface {
		if value.IsNil() {
			return sanitizeVisit{}, true
		}
		value = value.Elem()
	}

	if !value.IsValid() {
		return sanitizeVisit{}, true
	}

	switch value.Kind() {
	case reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return sanitizeVisit{}, true
		}
		ptr := value.Pointer()
		if ptr == 0 {
			return sanitizeVisit{}, true
		}

		tracked := sanitizeVisit{kind: value.Kind(), ptr: ptr}
		if _, ok := seen[tracked]; ok {
			return tracked, false
		}

		seen[tracked] = struct{}{}
		return tracked, true
	default:
		return sanitizeVisit{}, true
	}
}

// isSensitiveLogFieldKey reports whether key looks like a credential or
// PII identifier. Delegates to [sdkerrors.IsSensitiveFieldName] so the
// fragment list lives in exactly one place — the previous duplicated
// list in this package drifted from pkg/errors over time, producing
// inconsistent redaction depending on which layer touched a value first.
func isSensitiveLogFieldKey(key string) bool {
	return sdkerrors.IsSensitiveFieldName(strings.TrimSpace(key))
}

// mayContainSensitiveToken returns true when the string is plausibly carrying
// a credential payload. Both regex patterns require either an '=' / ':'
// assignment separator or a literal auth scheme prefix; if none of those
// markers are present we know the regex engine cannot match. Skipping the
// regex pass on every benign log line is the dominant win here — the byte
// scans are O(n) with very small constants.
func mayContainSensitiveToken(value string) bool {
	if strings.IndexByte(value, '=') >= 0 {
		return true
	}

	if strings.IndexByte(value, ':') >= 0 {
		return true
	}

	// Byte-level case-insensitive scan for "bearer" / "basic" prefixes,
	// bounded to the same window the redaction pass can actually inspect.
	// Allocating a full lower-cased copy of a multi-kilobyte log line just
	// to find a 6-byte literal showed up as wasted work under pprof on
	// retry-heavy workloads.
	end := len(value)
	if end > sanitizeMaxScanBytes {
		end = sanitizeMaxScanBytes
	}

	return containsFoldByte(value[:end], "bearer") || containsFoldByte(value[:end], "basic")
}

// containsFoldByte reports whether haystack contains needle using a
// byte-level ASCII case-insensitive comparison. needle must be ASCII
// lowercase; haystack is scanned without allocation. For the credential
// markers ("bearer", "basic") this is sufficient — neither scheme name
// contains non-ASCII characters.
func containsFoldByte(haystack, needle string) bool {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return false
	}

	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true

		for j := 0; j < len(needle); j++ {
			h := haystack[i+j]
			// Lowercase ASCII letter via OR-0x20. Safe because needle is
			// guaranteed lowercase: digits and symbols compare exactly,
			// uppercase letters fold to lowercase, lowercase passes through.
			if h >= 'A' && h <= 'Z' {
				h |= 0x20
			}

			if h != needle[j] {
				match = false
				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
