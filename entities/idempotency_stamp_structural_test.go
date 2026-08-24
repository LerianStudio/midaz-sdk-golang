package entities

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// unstampedWrites are the write operations that deliberately carry NO
// idempotency key, each with the reason it is safe.
//
// The list is literal rather than derived, for the same reason transitionHelpers
// is: a derived exemption would quietly absorb the next write someone forgets to
// stamp. Adding an entry here should cost a sentence of justification.
//
// Every exemption is TRACER-plane or compute-only. The ledger's money path has
// none: every write that can move a balance stamps a key.
//
// The three ledger transaction lifecycle actions (commit / cancel / revert, on
// both surfaces) are NOT listed here. They are not auto-idempotent — the ledger
// refuses a second commit of the same transaction, so an auto key would buy
// nothing — but they do carry a CALLER-supplied key through
// actionIdempotencyEditors, which satisfies the rule below.
var unstampedWrites = map[string]string{
	"ValidateTransaction": "the tracer dedups validations on body identifiers, and a validation moves nothing",
	"CreateReservation":   "the tracer dedups on the transactionId in the body, not on a header",

	"ConfirmReservation":              "keyed by the reservation named in the path; a replay confirms the same one",
	"ReleaseReservation":              "keyed by the reservation named in the path; a replay releases the same one",
	"ConfirmReservationByTransaction": "keyed by the transaction named in the path",
	"ReleaseReservationByTransaction": "keyed by the transaction named in the path",

	"ActivateRule":    "rule lifecycle is CONFIG, not money; no balance moves and the state is idempotent by value",
	"DeactivateRule":  "see ActivateRule",
	"DraftRule":       "see ActivateRule",
	"ActivateLimit":   "limit lifecycle is CONFIG, not money; same reasoning as the rule transitions",
	"DeactivateLimit": "see ActivateLimit",
	"DraftLimit":      "see ActivateLimit",

	"EstimateFeeCalculationV2": "fee estimation computes an answer and creates nothing",
	"CalculateBillingV2":       "billing calculation computes an answer and creates nothing",
}

// idempotencyHelpers are the calls that put a key on the wire. A facade that
// makes any of them satisfies the rule; which one is right depends on whether
// the write auto-generates a key or only carries a caller's.
//
// stampPreparers are the package-local helpers that resolve a key on a caller's
// behalf, so a facade delegating to one never names an idempotency helper
// itself. Their callers are credited on that promise, which is why the promise
// is checked directly below rather than taken on trust — deleting the resolution
// from one of these would otherwise silently unstamp every write that uses it.
var (
	idempotencyHelpers = map[string]bool{
		"idempotencyEditors":       true,
		"idempotencyEditorsTracer": true,
		"actionIdempotencyEditors": true,
		"applyIdempotency":         true,
		"resolveIdempotency":       true,
	}

	stampPreparers = map[string]bool{
		"prepareCreate": true,
	}
)

// TestEveryWriteStampsIdempotency is the structural invariant behind the SDK's
// central money-path guarantee: an unsafe request that leaves twice must not
// take effect twice.
//
// The generated clients do not inherit the legacy transport's header injection,
// and the auth round tripper only PRESERVES headers across a 401 replay — it
// never creates one. So a write that forgets to resolve a key leaves with none,
// and a transport-level retry after a timeout posts a second balance mutation.
// Nothing in either response says so.
//
// The two behavioural stamp tables sample one write per family. That proves the
// family is wired but says nothing about the other writes in it — and this epic
// roughly doubled the write surface, which is exactly when a sampling table
// stops keeping up. This asserts the property instead, on every write at once,
// derived from the generated clients at test time so a write added tomorrow is
// covered when it lands.
func TestEveryWriteStampsIdempotency(t *testing.T) {
	fset := token.NewFileSet()

	writeOps := writeOperations(t, fset)
	require.NotEmpty(t, writeOps, "found no generated write operations; the scan is broken, not the code")

	var unstamped []string

	stamped := 0
	preparerResolves := map[string]bool{}

	for name, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			if stampPreparers[fn.Name.Name] {
				preparerResolves[fn.Name.Name] = callsAny(fn, func(n string) bool { return idempotencyHelpers[n] })
			}

			switch complaint, counts := classifyWrite(fn, writeOps, fset, name); {
			case complaint != "":
				unstamped = append(unstamped, complaint)
			case counts:
				stamped++
			}
		}
	}

	// A stamp preparer takes no generated operation of its own, so the loop above
	// cannot see it. Its callers are credited purely on the promise that it
	// resolves a key, so that promise is checked here.
	for name := range stampPreparers {
		require.True(t, preparerResolves[name],
			"%s is credited as the idempotency wiring for the facade methods that delegate to it, "+
				"so it must resolve a key; found=%v", name, preparerResolves[name])
	}

	sort.Strings(unstamped)

	require.Empty(t, unstamped,
		"every unsafe request must resolve an idempotency key before it leaves, or a network retry "+
			"posts the write twice:\n  %s",
		strings.Join(unstamped, "\n  "))

	t.Logf("idempotency resolved on %d facade functions across %d generated write operations",
		stamped, len(writeOps))

	// A floor, so a scan that stops matching cannot read as success.
	require.Greater(t, stamped, 70,
		"expected the idempotency wiring on more than 70 facade functions; found %d", stamped)
}

// classifyWrite reports what one facade function does about idempotency: a
// complaint when it writes without resolving a key, otherwise whether it counts
// towards the stamped floor.
func classifyWrite(fn *ast.FuncDecl, writeOps map[string]bool, fset *token.FileSet, file string) (complaint string, counts bool) {
	ops := writeOperationsNamedBy(fn, writeOps)
	if len(ops) == 0 || allExempt(ops) {
		return "", false
	}

	if stampsIdempotency(fn) {
		return "", true
	}

	return funcLabel(fn) + " writes via " + strings.Join(ops, ", ") +
		" without an idempotency key (" + filepath.Base(file) + ":" +
		strconv.Itoa(fset.Position(fn.Pos()).Line) + ")", false
}

// writeOperationMethods are the HTTP methods that can change state. HEAD and GET
// are excluded; a repeated read is free.
var writeOperationMethods = regexp.MustCompile(`http\.NewRequest\("(POST|PUT|PATCH|DELETE)"`)

// writeOperations reads the generated clients and returns the operations whose
// request builder issues a state-changing method.
func writeOperations(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for path, file := range parseGoFiles(t, fset, dir) {
			collectWriteOperations(t, path, file, ops)
		}
	}

	return ops
}

// collectWriteOperations records every request builder in one generated file
// that issues a write method.
//
// The method is read from the SOURCE TEXT of the builder rather than from the
// AST, because oapi-codegen writes it as a bare string literal argument and
// matching that through the AST is more code for the same answer.
func collectWriteOperations(t *testing.T, path string, file *ast.File, ops map[string]bool) {
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
		body := src[fn.Body.Pos()-start : fn.Body.End()-start]

		if writeOperationMethods.MatchString(body) {
			ops[op] = true
		}
	}
}

// writeOperationsNamedBy returns the write operations a facade function names.
func writeOperationsNamedBy(fn *ast.FuncDecl, writeOps map[string]bool) []string {
	seen := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if op, ok := generatedOperation(sel.Sel.Name); ok && writeOps[op] {
			seen[op] = true
		}

		return true
	})

	return sortedKeys(seen)
}

// allExempt reports whether every operation the function names is on the
// deliberate no-key list. A function mixing an exempt write with a real one is
// NOT exempt.
func allExempt(ops []string) bool {
	for _, op := range ops {
		if _, ok := unstampedWrites[op]; !ok {
			return false
		}
	}

	return true
}

// stampsIdempotency reports whether the function routes through one of the
// helpers that put a key on the wire — directly, or via a stamp preparer.
func stampsIdempotency(fn *ast.FuncDecl) bool {
	return callsAny(fn, func(name string) bool {
		return idempotencyHelpers[name] || stampPreparers[name]
	})
}
