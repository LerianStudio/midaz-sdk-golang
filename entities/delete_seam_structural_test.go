package entities

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEveryDeleteRoutesThroughTheSharedSeam is the structural invariant behind
// the "a 2xx delete with an empty body is a success" fix.
//
// The defect it guards is not per-facade. The generated DELETE parser decodes
// the body as an error whenever the Content-Type contains "json", so a bodiless
// 204 stamped with a JSON content type by any proxy makes a SUCCESSFUL delete
// report "unexpected end of JSON input" — identically, in every facade. Fixing
// it once only stays fixed if no facade re-opens the private copy.
//
// The rule: a facade function that names a generated Delete* operation must hand
// the response to deleteResource. Reaching for the generated *WithResponse
// spelling is refused outright, because that spelling IS the broken parser.
//
// Derived from the source at test time rather than from a checked-in list, so a
// delete added tomorrow is covered when it lands rather than when someone
// remembers it.
//
// # What the scan matches, and why it is that loose
//
// It walks EVERY selector in a facade body and matches the name against the
// delete operations read out of the generated clients. It deliberately does not
// care what the selector hangs off, nor whether the selector is the thing being
// called.
//
// An earlier version cared about both, and a review wrote a compiling facade
// that called the banned parser and passed anyway. Three shapes walked straight
// through it: a client reached through a receiver field spelled anything other
// than "ledger" or "tracer"; an operation passed as a FUNCTION VALUE rather than
// called (the scan only looked at call targets); and a client hoisted into a
// local first, so the selector hung off a plain identifier. Each escape existed
// because the scan asserted something about the SHAPE of the call rather than
// about the operation being named. Matching the operation name against the
// generated set is both stricter and shorter — and it is what the sibling
// idempotency scan already does.
func TestEveryDeleteRoutesThroughTheSharedSeam(t *testing.T) {
	fset := token.NewFileSet()

	deleteOps := deleteOperations(t, fset)
	require.NotEmpty(t, deleteOps, "found no generated delete operations; the scan is broken, not the code")

	var offenders []string

	seamed := 0
	rawDecides := map[string]bool{}

	for name, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				offenders = append(offenders, packageScopeEscape(fset, name, decl, deleteOps)...)

				continue
			}

			parsed, raw := deleteOperationsNamedBy(fn.Body, deleteOps)
			if len(parsed) == 0 && len(raw) == 0 {
				continue
			}

			site := filepath.Base(name) + ":" + strconv.Itoa(fset.Position(fn.Pos()).Line)

			switch {
			case len(parsed) > 0:
				offenders = append(offenders, funcLabel(fn)+" ("+site+") names the generated parser "+
					strings.Join(parsed, ", ")+"; a bodiless 204 with a JSON content type fails there")
			case allStatusDeciding(raw):
				// The two tracer deletes predate the seam and decide on the status
				// inline. Credited on that promise, so the promise is checked.
				rawDecides[raw[0]] = decidesOnStatus(fn)
				seamed++
			case !callsAny(fn, func(n string) bool { return n == "deleteResource" }):
				offenders = append(offenders, funcLabel(fn)+" ("+site+") reaches "+
					strings.Join(raw, ", ")+" without deleteResource")
			default:
				seamed++
			}
		}
	}

	sort.Strings(offenders)

	require.Empty(t, offenders,
		"every delete must decide success on the status alone, through deleteResource:\n  %s",
		strings.Join(offenders, "\n  "))

	for op := range statusDecidingDeletes {
		require.True(t, rawDecides[op],
			"%s is exempted from deleteResource because it decides success on the status itself, "+
				"so it must call isSuccess; found=%v", op, rawDecides[op])
	}

	// A floor, so a scan that stops matching cannot read as success. 29 today:
	// 27 ledger deletes through deleteResource plus the two tracer deletes that
	// decide inline. Adding a delete raises it; losing one fails here.
	require.GreaterOrEqual(t, seamed, 29,
		"expected the delete seam on at least 29 facade functions (27 ledger + 2 tracer); found %d", seamed)
}

// packageScopeEscape reports a generated delete operation named OUTSIDE any
// function body, which is how a facade reaches the banned parser without any
// function in the package appearing to name it.
//
// Both scans in this package walked function bodies only, and two compiling
// shapes went straight through:
//
//	var escapeDeleteAccount = (*genledger.ClientWithResponses).DeleteAccountWithResponse
//
//	var escapeDeletes = map[string]func(...) (*genledger.DeleteAccountResponse, error){
//	    "account": func(c *genledger.ClientWithResponses, ...) (...) {
//	        return c.DeleteAccountWithResponse(ctx, orgID, ledgerID, id)
//	    },
//	}
//
// In both, the facade then calls a plain identifier — escapeDeleteAccount(...)
// or escapeDeletes["account"](...) — which carries no operation name, so the
// per-function accounting sees nothing to check. Both were written against the
// real generated client, confirmed to leave this test AND
// TestEveryPathParameterOperationIsGuarded green, and deleted once this sweep
// failed on them.
//
// A package-level var, const or type cannot name a generated delete operation
// for any innocent reason, so naming one is the whole signal — no seam is
// credited, the declaration is simply refused.
//
// Known ceiling: reflection is out of AST reach. A client method resolved by
// name at runtime is invisible to any scan of this kind, in this test and in its
// sibling. Nothing in this package does that, and the scans do not pretend to
// cover it.
func packageScopeEscape(fset *token.FileSet, path string, decl ast.Decl, deleteOps map[string]bool) []string {
	parsed, raw := deleteOperationsNamedBy(decl, deleteOps)
	if len(parsed) == 0 && len(raw) == 0 {
		return nil
	}

	return []string{"the package-level declaration at " + filepath.Base(path) + ":" +
		strconv.Itoa(fset.Position(decl.Pos()).Line) + " names " +
		strings.Join(append(parsed, raw...), ", ") +
		"; a generated delete reached from package scope leaves every caller naming a bare " +
		"identifier, which no scan of function bodies can attribute to a delete"}
}

// statusDecidingDeletes are the deletes that do NOT route through deleteResource
// because they decide success on the status inline, the same way it does. Both
// are tracer-plane and both predate the seam.
//
// Literal rather than derived: a derived exemption would quietly absorb the next
// delete someone writes by hand. Each entry costs a reason.
var statusDecidingDeletes = map[string]string{
	"DeleteRule":  "tracer-plane; decides on isSuccess inline (rules_facade.go)",
	"DeleteLimit": "tracer-plane; decides on isSuccess inline (limits_facade.go)",
}

// allStatusDeciding reports whether every delete the function names is on the
// inline-decision list. A function mixing an exempt delete with a real one is
// NOT exempt.
func allStatusDeciding(ops []string) bool {
	for _, op := range ops {
		if _, ok := statusDecidingDeletes[op]; !ok {
			return false
		}
	}

	return len(ops) > 0
}

// decidesOnStatus reports whether the function judges the response by its
// status code rather than by decoding the body.
func decidesOnStatus(fn *ast.FuncDecl) bool {
	return callsAny(fn, func(name string) bool { return name == "isSuccess" })
}

// deleteOperations reads the generated clients and returns the operations whose
// request builder issues DELETE. Deriving the set is what lets the facade scan
// match on the operation NAME alone: any selector naming one of these is a
// delete, however the call around it is spelled.
func deleteOperations(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for path, file := range parseGoFiles(t, fset, dir) {
			collectDeleteOperations(t, path, file, ops)
		}
	}

	return ops
}

// deleteMethod matches a request builder that issues DELETE.
var deleteMethod = regexp.MustCompile(`http\.NewRequest\("DELETE"`)

// collectDeleteOperations records every request builder in one generated file
// that issues DELETE, by the same source-text reading collectWriteOperations
// uses: oapi-codegen writes the method as a bare string literal.
func collectDeleteOperations(t *testing.T, path string, file *ast.File, ops map[string]bool) {
	t.Helper()

	src := readFileForScan(t, path)

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv != nil {
			continue
		}

		op, ok := requestBuilderOperation(fn.Name.Name)
		if !ok {
			continue
		}

		start := file.FileStart
		if deleteMethod.MatchString(src[fn.Body.Pos()-start : fn.Body.End()-start]) {
			ops[op] = true
		}
	}
}

// deleteOperationsNamedBy splits the generated delete operations a node names
// into the ones reached through the generated parser (*WithResponse) and the
// ones reached raw.
//
// It takes any ast.Node — a function body, or a whole package-level declaration
// — so the same matching covers both scopes. It inspects SelectorExpr nodes
// rather than call targets, and asks nothing about the receiver. A generated
// delete operation cannot be NAMED for any innocent reason, so naming one is the
// whole signal — whether it is called on f.ledger, on a hoisted local, on a
// differently-spelled field, handed to something else as a function value, or
// bound to a package-level var.
func deleteOperationsNamedBy(node ast.Node, deleteOps map[string]bool) (parsed, raw []string) {
	seenParsed := map[string]bool{}
	seenRaw := map[string]bool{}

	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		op, ok := generatedOperation(sel.Sel.Name)
		if !ok || !deleteOps[op] {
			return true
		}

		if strings.HasSuffix(sel.Sel.Name, "WithResponse") {
			seenParsed[op] = true
		} else {
			seenRaw[op] = true
		}

		return true
	})

	return sortedKeys(seenParsed), sortedKeys(seenRaw)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// readFileForScan reads a source file for the scans that need its raw text.
func readFileForScan(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(data)
}
