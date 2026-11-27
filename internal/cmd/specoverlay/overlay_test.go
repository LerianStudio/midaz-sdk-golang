package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const baseSpec = `openapi: 3.1.0
info:
  title: base
paths:
  /a:
    get:
      operationId: getA
  /b:
    get:
      operationId: getB
components:
  schemas:
    Error:
      type: object
    Thing:
      type: string
`

const fragmentSpec = `# local fragment
paths:
  /legacy/z:
    post:
      operationId: postZ
      responses:
        "201":
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LegacyZ'
  /legacy/y:
    post:
      operationId: postY
components:
  schemas:
    LegacyZ:
      type: object
    LegacyY:
      type: string
`

// mappingAt walks a mapping node along keys and returns the mapping found,
// or nil when a key is absent.
func mappingAt(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		var next *yaml.Node

		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == k {
				next = n.Content[i+1]
			}
		}

		if next == nil {
			return nil
		}

		n = next
	}

	return n
}

// keysAt returns the keys, in order, of the mapping at keys (nil if absent).
func keysAt(t *testing.T, doc string, keys ...string) []string {
	t.Helper()

	var n yaml.Node
	if err := yaml.Unmarshal([]byte(doc), &n); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(n.Content) == 0 {
		return nil
	}

	m := mappingAt(n.Content[0], keys...)
	if m == nil {
		return nil
	}

	var out []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, m.Content[i].Value)
	}

	return out
}

// generic decodes doc into plain Go values (aliases resolved).
func generic(t *testing.T, doc string) map[string]any {
	t.Helper()

	out := map[string]any{}
	if err := yaml.Unmarshal([]byte(doc), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return out
}

// sub returns the generic mapping under keys, or an empty one.
func sub(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		next, ok := m[k].(map[string]any)
		if !ok {
			return map[string]any{}
		}

		m = next
	}

	return m
}

// assertMerge checks a merge structurally: each section's keys are the base
// keys followed by the fragment keys, in order; every merged entry equals its
// source; and everything outside the two sections equals the base.
func assertMerge(t *testing.T, base, fragment, out string) {
	t.Helper()

	assertSection(t, base, fragment, out, "paths")
	assertSection(t, base, fragment, out, "components", "schemas")
	assertOutsideUnchanged(t, base, out)
}

// assertSection checks one merged section: keys in order (node decode) and
// every entry's value against its source (generic decode).
func assertSection(t *testing.T, base, fragment, out string, section ...string) {
	t.Helper()

	want := append(keysAt(t, base, section...), keysAt(t, fragment, section...)...)
	if got := keysAt(t, out, section...); !reflect.DeepEqual(got, want) {
		t.Errorf("%v keys = %q, want %q", section, got, want)
	}

	gotValues := sub(generic(t, out), section...)
	for src, doc := range map[string]string{"base": base, "fragment": fragment} {
		for k, v := range sub(generic(t, doc), section...) {
			if !reflect.DeepEqual(gotValues[k], v) {
				t.Errorf("%v %q from the %s changed: got %#v, want %#v", section, k, src, gotValues[k], v)
			}
		}
	}
}

// assertOutsideUnchanged checks that everything but paths and
// components.schemas equals the base.
func assertOutsideUnchanged(t *testing.T, base, out string) {
	t.Helper()

	b, o := generic(t, base), generic(t, out)
	for k, v := range b {
		if k == "paths" || k == "components" {
			continue
		}

		if !reflect.DeepEqual(o[k], v) {
			t.Errorf("top-level %q changed: got %#v, want %#v", k, o[k], v)
		}
	}

	for k, v := range sub(b, "components") {
		if k != "schemas" && !reflect.DeepEqual(sub(o, "components")[k], v) {
			t.Errorf("components.%s changed", k)
		}
	}
}

func TestOverlay(t *testing.T) {
	tests := []struct {
		name        string
		base        string
		fragment    string
		mustContain []string
	}{
		{
			name:        "paths and schemas are appended after the base entries in fragment order",
			base:        baseSpec,
			fragment:    fragmentSpec,
			mustContain: []string{"$ref: '#/components/schemas/LegacyZ'"},
		},
		{
			name:     "fragment with paths only leaves the schemas untouched",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get:\n      operationId: getZ\n",
		},
		{
			name:     "fragment with schemas only leaves the paths untouched",
			base:     baseSpec,
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
		},
		{
			name:     "base without components.schemas gets the section created",
			base:     "openapi: 3.1.0\npaths:\n  /a:\n    get: {}\ncomponents:\n  securitySchemes:\n    bearer:\n      type: http\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
		},
		{
			name:     "base without components gets components.schemas created",
			base:     "openapi: 3.1.0\npaths:\n  /a:\n    get: {}\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
		},
		{
			name:     "base without paths gets paths created",
			base:     "openapi: 3.1.0\ncomponents:\n  schemas:\n    Error:\n      type: object\n",
			fragment: "paths:\n  /legacy/z:\n    get: {}\n",
		},
		{
			name:     "two parameters separated by different literals are different paths",
			base:     "openapi: 3.1.0\npaths:\n  /a/{x}/b/{y}:\n    get: {}\n",
			fragment: "paths:\n  /a/{z}/c/{w}:\n    get: {}\n",
		},
		{
			name:     "anchors and aliases in the base pass through",
			base:     "openapi: 3.1.0\nx-shared: &shared\n  type: object\npaths:\n  /a:\n    get: {}\ncomponents:\n  schemas:\n    Error: *shared\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
		},
		{
			name:     "a path whose parameters differ from every base path is not a collision",
			base:     "openapi: 3.1.0\npaths:\n  /r/{id}:\n    get: {}\n",
			fragment: "paths:\n  /r/{id}/confirm:\n    post: {}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Overlay([]byte(tt.base), []byte(tt.fragment))
			if err != nil {
				t.Fatalf("Overlay returned error: %v", err)
			}

			assertMerge(t, tt.base, tt.fragment, string(got))

			for _, want := range tt.mustContain {
				if !strings.Contains(string(got), want) {
					t.Errorf("output missing %q\ngot:\n%s", want, got)
				}
			}
		})
	}
}

func TestOverlayEmptyFragmentReturnsTheBaseUnchanged(t *testing.T) {
	bases := []string{baseSpec, "openapi: 3.1.0\npaths:\n  /a:\n    get: {}\n"}
	fragments := []string{
		"", "\n", "# only a comment\n", "~\n", "null\n", "{}\n",
		"paths: {}\n", "components: {}\n", "components:\n  schemas: {}\n", "paths: {}\ncomponents:\n  schemas: {}\n",
	}

	for _, base := range bases {
		for _, fragment := range fragments {
			got, err := Overlay([]byte(base), []byte(fragment))
			if err != nil {
				t.Fatalf("Overlay(%q) returned error: %v", fragment, err)
			}

			if string(got) != base {
				t.Errorf("Overlay(%q) changed the base\ngot:\n%s", fragment, got)
			}
		}
	}
}

func TestOverlayRefuses(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		fragment string
		wantErr  []string
		wantNot  []string
	}{
		{
			name:     "path collision",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: {}\n  /b:\n    get: {}\n",
			wantErr:  []string{`path "/b"`, "base"},
		},
		{
			name:     "path collision with a renamed path parameter",
			base:     "openapi: 3.1.0\npaths:\n  /reservations/{reservation_id}/confirm:\n    post: {}\n",
			fragment: "paths:\n  /reservations/{id}/confirm:\n    post: {}\n",
			wantErr:  []string{`"/reservations/{id}/confirm"`, `"/reservations/{reservation_id}/confirm"`, "base"},
		},
		{
			name:     "schema collision",
			base:     baseSpec,
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n    Error:\n      type: object\n",
			wantErr:  []string{`schema "Error"`, "base"},
		},
		{
			name:     "top-level section other than paths and components",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: {}\ninfo:\n  title: x\n",
			wantErr:  []string{`"info"`},
		},
		{
			name:     "components section other than schemas",
			base:     baseSpec,
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n  responses:\n    R:\n      description: x\n",
			wantErr:  []string{`"responses"`},
		},
		{
			name:     "duplicate path inside the fragment",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: {}\n  /legacy/z:\n    post: {}\n",
			wantErr:  []string{`"/legacy/z"`, "twice"},
		},
		{
			name:     "duplicate path inside the fragment spelled with other parameter names",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/{a}:\n    get: {}\n  /legacy/{b}:\n    post: {}\n",
			wantErr:  []string{`"/legacy/{a}"`, `"/legacy/{b}"`, "twice"},
		},
		{
			name:     "duplicate schema inside the fragment",
			base:     baseSpec,
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n    LegacyZ:\n      type: string\n",
			wantErr:  []string{`"LegacyZ"`, "twice"},
		},
		{
			name:     "duplicate schemas section inside the fragment components",
			base:     baseSpec,
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n  schemas:\n    LegacyY:\n      type: string\n",
			wantErr:  []string{`"schemas"`, "twice"},
		},
		{
			name:     "duplicate top-level section inside the fragment",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: {}\npaths:\n  /legacy/y:\n    get: {}\n",
			wantErr:  []string{`fragment: root key "paths" is declared twice`},
			wantNot:  []string{"in the fragment"},
		},
		{
			name:     "merge key in the fragment paths",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: {}\n  <<:\n    /a:\n      get: {}\n",
			wantErr:  []string{"fragment", "paths", "merge key", "line 4"},
		},
		{
			name:     "merge key in the fragment schemas",
			base:     baseSpec,
			fragment: "components:\n  schemas:\n    <<: {Error: {type: object}}\n",
			wantErr:  []string{"fragment", "schemas", "merge key"},
		},
		{
			name:     "non-scalar key in the fragment paths",
			base:     baseSpec,
			fragment: "paths:\n  ? [/x, /y]\n  : get: {}\n",
			wantErr:  []string{"fragment", "paths", "not a plain scalar", "line 2"},
		},
		{
			name:     "fragment root that is not a mapping",
			base:     baseSpec,
			fragment: "- paths\n",
			wantErr:  []string{"fragment", "mapping"},
		},
		{
			name:     "fragment paths that is not a mapping",
			base:     baseSpec,
			fragment: "paths:\n  - /legacy/z\n",
			wantErr:  []string{"fragment", "paths", "mapping"},
		},
		{
			name:     "fragment paths that is null",
			base:     baseSpec,
			fragment: "paths:\n",
			wantErr:  []string{"fragment", "paths", "mapping"},
		},
		{
			name:     "fragment components that is not a mapping",
			base:     baseSpec,
			fragment: "components: x\n",
			wantErr:  []string{"fragment", "components", "mapping"},
		},
		{
			name:     "base root that is not a mapping",
			base:     "- a\n",
			fragment: fragmentSpec,
			wantErr:  []string{"base", "mapping"},
		},
		{
			name:     "base paths that is not a mapping",
			base:     "openapi: 3.1.0\npaths: x\n",
			fragment: fragmentSpec,
			wantErr:  []string{"base", "paths", "mapping"},
		},
		{
			name:     "base schemas that is not a mapping",
			base:     "openapi: 3.1.0\ncomponents:\n  schemas:\n    - x\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
			wantErr:  []string{"base", "schemas", "mapping"},
		},
		{
			name:     "base declaring paths twice",
			base:     "openapi: 3.1.0\npaths:\n  /a:\n    get: {}\npaths:\n  /legacy/z:\n    get: {}\n",
			fragment: "paths:\n  /legacy/z:\n    get: {}\n",
			wantErr:  []string{"base", `"paths"`, "twice"},
		},
		{
			name:     "base declaring components twice",
			base:     "openapi: 3.1.0\ncomponents:\n  schemas: {}\ncomponents:\n  schemas: {}\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
			wantErr:  []string{"base", `"components"`, "twice"},
		},
		{
			name:     "base declaring schemas twice",
			base:     "openapi: 3.1.0\ncomponents:\n  schemas:\n    A: {}\n  schemas:\n    B: {}\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
			wantErr:  []string{"base", `"schemas"`, "twice"},
		},
		{
			name:     "merge key in the base schemas",
			base:     "openapi: 3.1.0\nx-s: &s\n  LegacyZ:\n    type: object\ncomponents:\n  schemas:\n    <<: *s\n    Error:\n      type: object\n",
			fragment: "components:\n  schemas:\n    LegacyZ:\n      type: object\n",
			wantErr:  []string{"base", "schemas", "merge key"},
		},
		{
			name:     "non-scalar key in the base paths",
			base:     "openapi: 3.1.0\npaths:\n  ? [/x, /y]\n  : get: {}\n",
			fragment: "paths:\n  /legacy/z:\n    get: {}\n",
			wantErr:  []string{"base", "paths", "not a plain scalar", "line 3"},
		},
		{
			name:     "merge key in the base paths",
			base:     "openapi: 3.1.0\nx-p: &p\n  /legacy/z:\n    get: {}\npaths:\n  <<: *p\n  /a:\n    get: {}\n",
			fragment: "paths:\n  /legacy/z:\n    get: {}\n",
			wantErr:  []string{"base", "paths", "merge key"},
		},
		{
			name:     "alias in the fragment",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: &op\n      operationId: getZ\n  /legacy/y:\n    get: *op\n",
			wantErr:  []string{"anchor"},
		},
		{
			name:     "anchor without alias in the fragment",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: &op\n      operationId: getZ\n",
			wantErr:  []string{"anchor"},
		},
		{
			name:     "multi-document fragment",
			base:     baseSpec,
			fragment: "paths:\n  /legacy/z:\n    get: {}\n---\npaths:\n  /legacy/y:\n    get: {}\n",
			wantErr:  []string{"fragment", "one YAML document"},
		},
		{
			name:     "multi-document base",
			base:     baseSpec + "---\nopenapi: 3.1.0\n",
			fragment: fragmentSpec,
			wantErr:  []string{"base", "one YAML document"},
		},
		{
			name:     "empty base",
			base:     "",
			fragment: fragmentSpec,
			wantErr:  []string{"base", "one YAML document"},
		},
		{
			name:     "unparseable fragment",
			base:     baseSpec,
			fragment: "paths: [\n",
			wantErr:  []string{"fragment", "parse"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRefusal(t, tt.base, tt.fragment, tt.wantErr, tt.wantNot)
		})
	}
}

// assertRefusal requires an error mentioning every wantErr and none of
// wantNot, with no output.
func assertRefusal(t *testing.T, base, fragment string, wantErr, wantNot []string) {
	t.Helper()

	got, err := Overlay([]byte(base), []byte(fragment))
	if err == nil {
		t.Fatalf("Overlay returned no error\ngot:\n%s", got)
	}

	if got != nil {
		t.Errorf("Overlay returned output alongside the error:\n%s", got)
	}

	for _, want := range wantErr {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}

	for _, bad := range wantNot {
		if strings.Contains(err.Error(), bad) {
			t.Errorf("error %q should not contain %q", err.Error(), bad)
		}
	}
}

const (
	goldenBase     = "openapi: 3.1.0\npaths:\n  /a/{id}:\n    get: {}\ncomponents:\n  schemas:\n    E:\n      type: object\n"
	goldenFragment = "# fragment\npaths:\n  /z:\n    post:\n      operationId: postZ\n      responses:\n        \"201\":\n          $ref: \"#/components/schemas/Z\"\ncomponents:\n  schemas:\n    Z:\n      type: string\n"
	goldenOutput   = `openapi: 3.1.0
paths:
    /a/{id}:
        get: {}
    /z:
        post:
            operationId: postZ
            responses:
                "201":
                    $ref: "#/components/schemas/Z"
components:
    schemas:
        E:
            type: object
        Z:
            type: string
`
)

func TestPathShape(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"{id}", "{}"},
		{"{ id }", "{}"},
		{"{}", "{}"},
		{"/a/{x}.{y}", "/a/{}.{}"},
		{"/a/{x}{y}", "/a/{}{}"},
		{"/a/{x}/b/{y}", "/a/{}/b/{}"},
		{"/f{bar}baz", "/f{}baz"},
		{"/a/{x", "/a/{x"},
		{"/a/b", "/a/b"},
	}

	for _, tt := range tests {
		if got := pathShape(tt.in); got != tt.want {
			t.Errorf("pathShape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOverlayDeterministic(t *testing.T) {
	for i := range 2 {
		got, err := Overlay([]byte(goldenBase), []byte(goldenFragment))
		if err != nil {
			t.Fatalf("run %d: Overlay error: %v", i+1, err)
		}

		if string(got) != goldenOutput {
			t.Errorf("run %d: output differs from the golden output\ngot:\n%s\nwant:\n%s", i+1, got, goldenOutput)
		}
	}
}

// writeFile writes content into dir/name and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}

	return p
}

func TestRunWritesTheMergedSpec(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.yaml", baseSpec)
	fragment := writeFile(t, dir, "fragment.yaml", fragmentSpec)
	out := filepath.Join(dir, "out.yaml")

	var stderr bytes.Buffer
	if code := run([]string{"specoverlay", base, fragment, out}, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	assertMerge(t, baseSpec, fragmentSpec, string(got))
}

// assertRunFails runs the command and requires exit 1, a stderr mentioning
// want, and no output file.
func assertRunFails(t *testing.T, base, fragment, want string) {
	t.Helper()

	out := filepath.Join(filepath.Dir(base), "out.yaml")

	var stderr bytes.Buffer
	if code := run([]string{"specoverlay", base, fragment, out}, &stderr); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q does not mention %q", stderr.String(), want)
	}

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output exists after a failure (stat err %v)", err)
	}
}

func TestRunCollisionWritesNothingAndNamesTheKey(t *testing.T) {
	dir := t.TempDir()
	assertRunFails(t, writeFile(t, dir, "base.yaml", baseSpec), writeFile(t, dir, "fragment.yaml", "paths:\n  /a:\n    get: {}\n"), `path "/a"`)
}

func TestRunInvalidYAMLWritesNothing(t *testing.T) {
	dir := t.TempDir()
	assertRunFails(t, writeFile(t, dir, "base.yaml", baseSpec), writeFile(t, dir, "fragment.yaml", "paths: [\n"), "parse yaml")
}

func TestRunCollisionLeavesAnExistingOutputUntouched(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.yaml", baseSpec)
	fragment := writeFile(t, dir, "fragment.yaml", "components:\n  schemas:\n    Error:\n      type: object\n")
	out := writeFile(t, dir, "out.yaml", "previous\n")

	var stderr bytes.Buffer
	if code := run([]string{"specoverlay", base, fragment, out}, &stderr); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if string(got) != "previous\n" {
		t.Errorf("output changed after a failed merge: %q", got)
	}
}

func TestRunUnreadableInputFails(t *testing.T) {
	dir := t.TempDir()
	assertRunFails(t, filepath.Join(dir, "missing-base.yaml"), writeFile(t, dir, "fragment.yaml", fragmentSpec), "missing-base.yaml")
	assertRunFails(t, writeFile(t, dir, "base.yaml", baseSpec), filepath.Join(dir, "missing-fragment.yaml"), "missing-fragment.yaml")
}

func TestRunUnwritableOutputFails(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.yaml", baseSpec)
	fragment := writeFile(t, dir, "fragment.yaml", fragmentSpec)
	out := filepath.Join(dir, "no-such-dir", "out.yaml")

	var stderr bytes.Buffer
	if code := run([]string{"specoverlay", base, fragment, out}, &stderr); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "write") {
		t.Errorf("stderr %q does not report the write", stderr.String())
	}
}

func TestRunWrongArgumentCountPrintsUsage(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"specoverlay", "only-one"}, &stderr); code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}

	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("stderr %q has no usage line", stderr.String())
	}
}
