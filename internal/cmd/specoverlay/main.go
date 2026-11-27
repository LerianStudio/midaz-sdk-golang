// Command specoverlay appends a local OpenAPI fragment to a server spec, so a
// client can be generated for operations the server spec no longer declares
// without editing the server spec itself.
//
// The fragment may carry only two sections: `paths` and `components.schemas`.
// Their keys are appended at the END of the base's `paths` and
// `components.schemas`, in fragment order; nothing in the base is modified,
// reordered or removed. A fragment can only ADD: a path or schema the base
// already declares is a collision, and the merge fails naming the key instead
// of overwriting the server contract.
//
// Paths are compared by shape: every `{...}` parameter segment counts as the
// same, on both sides, so `/limits/{id}` in the fragment collides with
// `/limits/{limit_id}` in the base (servers rename path parameters between
// versions, and both keys name one route). The error names both spellings.
// The fragment's own duplicate-path check uses the same comparison. Schema
// names are compared exactly.
//
// Edge cases, decided here and pinned by overlay_test.go:
//
//   - Empty fragment (no document, a comment-only file, `null`, `{}`, or
//     `paths`/`components`/`schemas` written as empty mappings `{}`): the
//     base is returned byte for byte, no error.
//   - A section present with a null or other non-mapping value (`paths:`
//     with nothing after it, in the fragment or the base): error, not empty.
//   - Base without `paths`, without `components`, or with `components` but no
//     `schemas`: the missing section is created at the end of its parent when
//     the fragment has entries for it. Nothing is created for an empty section.
//   - A node that must be a mapping and is not (either root, `paths`,
//     `components`, `schemas`): error naming the side and the section.
//   - A key the fragment declares twice (top-level, under `components`, under
//     `paths` or under `schemas`), or a base declaring `paths`, `components`
//     or `schemas` twice: error naming the key. The YAML decoder does not
//     reject duplicate keys in a node tree, so the check is explicit.
//   - A fragment key that is not a plain scalar or is a YAML merge key (`<<`),
//     and a merge key in the base `paths` or `schemas`: error naming the
//     section and line. A merge key pulls in entries the collision check
//     cannot see.
//   - Anchors or aliases anywhere in the fragment: error. Moving an aliased
//     node into another document would silently change what it refers to.
//     Anchors in the base pass through untouched.
//   - More than one YAML document in either input, or no document in the
//     base: error. A second document would otherwise be dropped in silence.
//
// The output is the base node tree re-marshalled with yaml.v3, like
// specdowngrade's, and deterministic for the same inputs. The command builds
// the whole output in memory before writing, so a failed merge leaves the
// output path untouched.
//
// Usage: specoverlay <base.yaml> <fragment.yaml> <output.yaml>
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

const (
	sectionPaths      = "paths"
	sectionComponents = "components"
	sectionSchemas    = "schemas"
)

var (
	errAlreadyInBase = errors.New("is already declared by the base spec")
	errNotAllowed    = errors.New("is not allowed in a fragment")
	errDeclaredTwice = errors.New("is declared twice")
	errNotPlainKey   = errors.New("is not a plain scalar")
	errMergeKey      = errors.New("is a merge key (<<), whose entries the collision check cannot see")
	errNotMapping    = errors.New("is not a mapping")
	errDocuments     = errors.New("must hold exactly one YAML document")
	errAnchor        = errors.New("anchors and aliases are not supported in a fragment")
)

// pathTemplate matches one path parameter segment, e.g. {limit_id}.
var pathTemplate = regexp.MustCompile(`\{[^}]*\}`)

// pathShape erases the parameter names of a path key, so /limits/{id} and
// /limits/{limit_id} compare equal: they are the same route to a server.
func pathShape(key string) string {
	return pathTemplate.ReplaceAllString(key, "{}")
}

// exact compares keys as written (schema names, section names).
func exact(key string) string { return key }

// entry is one key/value pair of a mapping node.
type entry struct {
	key   *yaml.Node
	value *yaml.Node
}

// Overlay appends the fragment's `paths` and `components.schemas` entries to
// the base spec and returns the merged spec. On any error it returns nil
// output; the error names the offending key or section.
func Overlay(base, fragment []byte) ([]byte, error) {
	baseDoc, found, err := parseDocument("base", base)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("base %w, found 0", errDocuments)
	}

	baseRoot := baseDoc.Content[0]
	if baseRoot.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("base: root %w", errNotMapping)
	}

	paths, schemas, err := fragmentEntries(fragment)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 && len(schemas) == 0 {
		return base, nil
	}

	if err := appendEntries(baseRoot, paths, schemas); err != nil {
		return nil, err
	}

	out, err := yaml.Marshal(baseDoc)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	return out, nil
}

// parseDocument decodes data into a single document node. found is false when
// data holds no document; more than one document is an error.
func parseDocument(side string, data []byte) (doc *yaml.Node, found bool, err error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var docs []*yaml.Node

	for {
		var d yaml.Node

		decErr := dec.Decode(&d)
		if errors.Is(decErr, io.EOF) {
			break
		}

		if decErr != nil {
			return nil, false, fmt.Errorf("%s: parse yaml: %w", side, decErr)
		}

		docs = append(docs, &d)
	}

	switch len(docs) {
	case 0:
		return nil, false, nil
	case 1:
		return docs[0], true, nil
	default:
		return nil, false, fmt.Errorf("%s %w, found %d", side, errDocuments, len(docs))
	}
}

// fragmentEntries validates the fragment and returns its path and schema
// entries in fragment order. An empty fragment yields no entries.
func fragmentEntries(fragment []byte) (paths, schemas []entry, err error) {
	doc, found, err := parseDocument("fragment", fragment)
	if err != nil || !found {
		return nil, nil, err
	}

	if err := refuseAnchors(doc); err != nil {
		return nil, nil, err
	}

	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return nil, nil, nil
	}

	sections, err := mappingEntries("root", root, exact)
	if err != nil {
		return nil, nil, err
	}

	for _, s := range sections {
		switch s.key.Value {
		case sectionPaths:
			if paths, err = mappingEntries(sectionPaths, s.value, pathShape); err != nil {
				return nil, nil, err
			}
		case sectionComponents:
			if schemas, err = componentSchemas(s.value); err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, fmt.Errorf("fragment: top-level section %q %w (want %s or %s)",
				s.key.Value, errNotAllowed, sectionPaths, sectionComponents)
		}
	}

	return paths, schemas, nil
}

// componentSchemas returns the entries of the fragment's components.schemas,
// refusing any other components section.
func componentSchemas(components *yaml.Node) ([]entry, error) {
	sections, err := mappingEntries(sectionComponents, components, exact)
	if err != nil {
		return nil, err
	}

	var schemas []entry

	for _, s := range sections {
		if s.key.Value != sectionSchemas {
			return nil, fmt.Errorf("fragment: components section %q %w (want %s)", s.key.Value, errNotAllowed, sectionSchemas)
		}

		if schemas, err = mappingEntries(sectionComponents+"."+sectionSchemas, s.value, exact); err != nil {
			return nil, err
		}
	}

	return schemas, nil
}

// mappingEntries returns the entries of a fragment mapping node. It refuses a
// node that is not a mapping, a key that is not a plain scalar or is a merge
// key, and two keys equal under same (the path shape for paths, exact text
// otherwise).
func mappingEntries(section string, n *yaml.Node, same func(string) string) ([]entry, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("fragment: %s %w", section, errNotMapping)
	}

	if err := refuseOddKeys("fragment", section, n); err != nil {
		return nil, err
	}

	seen := make(map[string]string, len(n.Content)/2)
	out := make([]entry, 0, len(n.Content)/2)

	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if first, dup := seen[same(k.Value)]; dup {
			if first != k.Value {
				return nil, fmt.Errorf("fragment: %s key %q %w (also as %q)", section, k.Value, errDeclaredTwice, first)
			}

			return nil, fmt.Errorf("fragment: %s key %q %w", section, k.Value, errDeclaredTwice)
		}

		seen[same(k.Value)] = k.Value
		out = append(out, entry{key: k, value: n.Content[i+1]})
	}

	return out, nil
}

// refuseOddKeys refuses, in a mapping, a key that is not a plain scalar or
// that is a YAML merge key: either would let entries reach the output without
// the collision check seeing their names.
func refuseOddKeys(side, section string, n *yaml.Node) error {
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]

		switch {
		case k.Kind != yaml.ScalarNode:
			return fmt.Errorf("%s: %s key at line %d %w", side, section, k.Line, errNotPlainKey)
		case k.Tag == "!!merge":
			return fmt.Errorf("%s: %s key at line %d %w", side, section, k.Line, errMergeKey)
		}
	}

	return nil
}

// refuseAnchors walks the fragment and refuses any anchor or alias.
func refuseAnchors(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return fmt.Errorf("fragment: line %d: %w", n.Line, errAnchor)
	}

	for _, c := range n.Content {
		if err := refuseAnchors(c); err != nil {
			return err
		}
	}

	return nil
}

// appendEntries checks every fragment key against the base, then appends the
// entries. Nothing is appended unless every check passes.
func appendEntries(root *yaml.Node, paths, schemas []entry) error {
	basePaths, _, err := section(root, "root", sectionPaths)
	if err != nil {
		return err
	}

	components, hasComponents, err := section(root, "root", sectionComponents)
	if err != nil {
		return err
	}

	var baseSchemas *yaml.Node
	if hasComponents {
		if baseSchemas, _, err = section(components, sectionComponents, sectionSchemas); err != nil {
			return err
		}
	}

	if err := refuseCollisions("path", sectionPaths, basePaths, paths, pathShape); err != nil {
		return err
	}

	if err := refuseCollisions("schema", sectionComponents+"."+sectionSchemas, baseSchemas, schemas, exact); err != nil {
		return err
	}

	if len(paths) > 0 {
		if basePaths == nil {
			basePaths = addMapping(root, sectionPaths)
		}

		appendTo(basePaths, paths)
	}

	if len(schemas) > 0 {
		if components == nil {
			components = addMapping(root, sectionComponents)
		}

		if baseSchemas == nil {
			baseSchemas = addMapping(components, sectionSchemas)
		}

		appendTo(baseSchemas, schemas)
	}

	return nil
}

// section returns the base mapping under key in m (parent names m in
// errors). found is false when the key is absent; a key declared twice and a
// value that is not a mapping are errors.
func section(m *yaml.Node, parent, key string) (v *yaml.Node, found bool, err error) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != key {
			continue
		}

		if found {
			return nil, false, fmt.Errorf("base: %s key %q %w", parent, key, errDeclaredTwice)
		}

		v, found = m.Content[i+1], true
	}

	if found && v.Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("base: %s %w", key, errNotMapping)
	}

	return v, found, nil
}

// refuseCollisions fails on the first fragment key the base mapping already
// declares, comparing keys under same; the error names both spellings. A nil
// base mapping declares nothing. A merge key in the base mapping is an error:
// the names it pulls in are invisible to this check.
func refuseCollisions(kind, section string, base *yaml.Node, entries []entry, same func(string) string) error {
	if base == nil {
		return nil
	}

	if err := refuseOddKeys("base", section, base); err != nil {
		return err
	}

	declared := make(map[string]string, len(base.Content)/2)
	for i := 0; i+1 < len(base.Content); i += 2 {
		declared[same(base.Content[i].Value)] = base.Content[i].Value
	}

	for _, e := range entries {
		if b, ok := declared[same(e.key.Value)]; ok {
			return fmt.Errorf("%s %q %w as %q", kind, e.key.Value, errAlreadyInBase, b)
		}
	}

	return nil
}

// addMapping appends `key: {}` (block style) to m and returns the new mapping.
func addMapping(m *yaml.Node, key string) *yaml.Node {
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)

	return v
}

// appendTo appends the entries to the end of mapping m, in order.
func appendTo(m *yaml.Node, entries []entry) {
	for _, e := range entries {
		m.Content = append(m.Content, e.key, e.value)
	}
}

// wantArgs is the required argument count: program name plus the base,
// fragment and output spec paths.
const wantArgs = 4

// run is main without the process exit, so the exit codes and the
// write-nothing-on-failure behaviour are testable. It returns 2 on a usage
// error and 1 on any other failure.
func run(args []string, stderr io.Writer) int {
	if len(args) != wantArgs {
		_, _ = fmt.Fprintln(stderr, "usage: specoverlay <base.yaml> <fragment.yaml> <output.yaml>")
		return 2
	}

	// The three paths are fixed, repo-relative build-time inputs from
	// scripts/generate-clients.sh (api/<plane>.openapi.yaml, the plane's
	// fragment under api/, and an ephemeral mktemp output). This is a
	// developer/CI-only codegen tool under internal/cmd; the paths are never
	// user- or network-supplied.
	base, err := os.ReadFile(args[1]) //#nosec G703 -- fixed build-time path from generate-clients.sh, not user/network controlled
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read %s: %v\n", args[1], err)
		return 1
	}

	fragment, err := os.ReadFile(args[2]) //#nosec G703 -- fixed build-time path from generate-clients.sh, not user/network controlled
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read %s: %v\n", args[2], err)
		return 1
	}

	out, err := Overlay(base, fragment)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "overlay: %v\n", err)
		return 1
	}

	// The output is an ephemeral intermediate in a mktemp dir, consumed by
	// specdowngrade and deleted on exit (never committed), so 0600.
	if err := os.WriteFile(args[3], out, 0o600); err != nil { //#nosec G703 -- fixed build-time mktemp path from generate-clients.sh, not user/network controlled
		_, _ = fmt.Fprintf(stderr, "write %s: %v\n", args[3], err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(run(os.Args, os.Stderr))
}
