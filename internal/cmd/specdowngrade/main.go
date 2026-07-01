// Command specdowngrade rewrites a native OpenAPI 3.1 YAML spec into an
// equivalent 3.0.3 spec that oapi-codegen v2 can consume.
//
// oapi-codegen v2 does not support OAS 3.1 (it chokes on 3.1 nullable
// type-arrays). Three transforms bridge the gap for the Midaz specs:
//
//	(a) nullable type-arrays: `type: [X, "null"]` -> `type: X` + `nullable: true`.
//	(b) meaningless format: strip `format` where the resolved type is a scalar
//	    not in {string, number, integer}.
//	(c) base64 bodies: `contentEncoding: base64` on a string -> `format: byte`,
//	    the 3.0.3 idiom, so oapi-codegen emits `[]byte` (which encoding/json
//	    base64-decodes) rather than a raw, undecoded base64 string.
//
// It also sets `openapi: 3.0.3`. Other 3.1-only annotation keys that oapi-codegen
// (via kin-openapi) tolerates and that do not affect the generated Go —
// `examples` (plural arrays) and `contentMediaType` — are intentionally left as
// passthrough; converting them would change nothing downstream. The output is
// deterministic: the YAML node tree is walked in place, preserving key order, so
// it is committed and drift-checked in CI.
//
// Usage: specdowngrade <input-3.1.yaml> <output-3.0.3.yaml>
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// formatOKTypes are the scalar types that legitimately carry a `format`.
var formatOKTypes = map[string]bool{"string": true, "number": true, "integer": true}

// Downgrade transforms a 3.1 OpenAPI YAML document into a 3.0.3 one.
func Downgrade(in []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(in, &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	walk(&doc)
	setOpenAPIVersion(&doc)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}
	return out, nil
}

// setOpenAPIVersion forces the top-level `openapi` key to 3.0.3.
func setOpenAPIVersion(doc *yaml.Node) {
	root := doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "openapi" {
			v := root.Content[i+1]
			v.Value = "3.0.3"
			v.Tag = "!!str"
			v.Style = 0
			return
		}
	}
}

// walk applies the two schema transforms to every mapping node in the tree.
func walk(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			walk(c)
		}
	case yaml.MappingNode:
		resolvedType := collapseNullableType(n)
		stripMeaninglessFormat(n, resolvedType)
		bridgeBase64Encoding(n, resolvedType)
		// Recurse into values (Content is [key, value, key, value, ...]).
		for i := 1; i < len(n.Content); i += 2 {
			walk(n.Content[i])
		}
	}
}

// collapseNullableType rewrites `type: [X, "null"]` into `type: X` plus
// `nullable: true`. It returns the resolved scalar type ("" if type is absent
// or not collapsed to a single scalar).
func collapseNullableType(m *yaml.Node) string {
	typeVal := mapGet(m, "type")
	if typeVal == nil {
		return ""
	}
	if typeVal.Kind == yaml.ScalarNode {
		return typeVal.Value
	}
	if typeVal.Kind != yaml.SequenceNode {
		return ""
	}

	hasNull := false
	nonNull := make([]*yaml.Node, 0, len(typeVal.Content))
	for _, item := range typeVal.Content {
		if item.Kind == yaml.ScalarNode && item.Value == "null" {
			hasNull = true
			continue
		}
		nonNull = append(nonNull, item)
	}
	if !hasNull {
		return ""
	}

	mapSetBool(m, "nullable", true)

	if len(nonNull) == 1 {
		// Collapse the sequence node into the single scalar in place.
		single := nonNull[0]
		typeVal.Kind = single.Kind
		typeVal.Tag = single.Tag
		typeVal.Value = single.Value
		typeVal.Style = 0
		typeVal.Content = nil
		return single.Value
	}
	// Multiple non-null types remain a sequence (no scalar type to report).
	typeVal.Content = nonNull
	return ""
}

// stripMeaninglessFormat removes `format` when the resolved scalar type does
// not legitimately carry one.
func stripMeaninglessFormat(m *yaml.Node, resolvedType string) {
	if resolvedType == "" || formatOKTypes[resolvedType] {
		return
	}
	if mapGet(m, "format") == nil {
		return
	}
	out := make([]*yaml.Node, 0, len(m.Content))
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "format" {
			continue
		}
		out = append(out, m.Content[i], m.Content[i+1])
	}
	m.Content = out
}

// bridgeBase64Encoding rewrites the 3.1 `contentEncoding: base64` annotation on
// a string schema into the 3.0.3 idiom `format: byte`, then drops the now-
// redundant contentEncoding key. This makes oapi-codegen emit `[]byte` (which
// encoding/json base64-decodes) instead of a raw base64 string. Scoped to
// string types so it never touches non-schema or non-string mappings.
func bridgeBase64Encoding(m *yaml.Node, resolvedType string) {
	if resolvedType != "string" {
		return
	}
	enc := mapGet(m, "contentEncoding")
	if enc == nil || enc.Kind != yaml.ScalarNode || enc.Value != "base64" {
		return
	}
	mapSetString(m, "format", "byte")
	mapDelete(m, "contentEncoding")
}

// mapGet returns the value node for key in a mapping node, or nil.
func mapGet(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mapSetBool sets key to a boolean value, updating in place if present.
func mapSetBool(m *yaml.Node, key string, val bool) {
	valStr := "false"
	if val {
		valStr = "true"
	}
	if v := mapGet(m, key); v != nil {
		v.Kind = yaml.ScalarNode
		v.Tag = "!!bool"
		v.Value = valStr
		v.Content = nil
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: valStr},
	)
}

// mapSetString sets key to a string value, updating in place if present.
func mapSetString(m *yaml.Node, key, val string) {
	if v := mapGet(m, key); v != nil {
		v.Kind = yaml.ScalarNode
		v.Tag = "!!str"
		v.Value = val
		v.Style = 0
		v.Content = nil
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val},
	)
}

// mapDelete removes key (and its value) from a mapping node if present.
func mapDelete(m *yaml.Node, key string) {
	out := make([]*yaml.Node, 0, len(m.Content))
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			continue
		}
		out = append(out, m.Content[i], m.Content[i+1])
	}
	m.Content = out
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: specdowngrade <input-3.1.yaml> <output-3.0.3.yaml>")
		os.Exit(2)
	}
	in, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	out, err := Downgrade(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "downgrade: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", os.Args[2], err)
		os.Exit(1)
	}
}
