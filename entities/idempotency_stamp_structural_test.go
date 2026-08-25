package entities

import (
	"go/ast"
	"go/token"
	"net/http"
	"path/filepath"
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
//
// # Where the write universe comes from
//
// writeOperations DERIVES it by parsing ../internal/genledger and
// ../internal/gentracer and keeping every request builder whose body issues
// POST, PUT, PATCH or DELETE. Nothing here is hand-maintained, so there is no
// list to forget to update and the stamped floor below is a backstop against the
// SCAN breaking, not against a missed entry. It is 122 today, which is exactly
// the 60 POST + 32 PATCH + 29 DELETE + 1 PUT the generated clients issue.
//
// The method is read out of the builder's AST by requestBuilderMethod, which
// accepts the two spellings that denote a method constant and FAILS on anything
// else. That is a change of stance: the previous reading matched the method as a
// string literal in the builder's source text and merely documented that a
// builder spelling it any other way would drop out of this universe silently.
// The oapi-codegen v2.7 upgrade made that concrete — every builder moved from
// "POST" to http.MethodPost and this universe came back EMPTY — so the residual
// is now closed at the reading instead of restated here.
//
// # Credit attaches to the OCCURRENCE, never to the function
//
// The previous version asked stampsIdempotency(fn): a FUNCTION-LEVEL boolean.
// One stamped write credited every write the function named, because the boolean
// had already been satisfied somewhere in the body — so a facade that resolved a
// key for one create and posted a second write beside it with none passed. Live
// exposure is zero only because every facade method names one write today, and
// this is the same shape a review turned into an unguarded destructive request in
// the sibling path-guard scan.
//
// A write OPERATION CALL is now accounted individually, and every unaccounted
// call is reported, same-named or not.
func TestEveryWriteStampsIdempotency(t *testing.T) {
	fset := token.NewFileSet()

	writeOps := writeOperations(t, fset)
	require.NotEmpty(t, writeOps, "found no generated write operations; the scan is broken, not the code")

	scan := &idempotencyStampScan{fset: fset, writeOps: writeOps}

	var unstamped []string

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

			unstamped = append(unstamped, scan.offences(name, fn)...)
		}
	}

	// A stamp preparer takes no generated operation of its own, so the loop above
	// cannot see it. Its callers are credited purely on the promise that it
	// resolves a key, so that promise is checked here. This one IS a
	// function-level question — does this helper resolve a key anywhere in its
	// body — because the helper exists to do nothing else.
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

	t.Logf("idempotency resolved on %d write call sites across %d generated write operations",
		scan.stamped, len(writeOps))

	// A floor, so a scan that stops matching cannot read as success. It counts
	// CALL SITES rather than functions, which is the same 109 while every facade
	// names one write, and the honest number the moment one names two.
	//
	// The floor is the LIVE count, not a round number below it. A floor of 70
	// against 109 live sites let a third of the stamped surface disappear without
	// failing anything — which is the regression a floor exists to catch, not to
	// tolerate. Adding a stamped write raises this number; removing one should
	// fail here and be argued for, not absorbed.
	require.GreaterOrEqual(t, scan.stamped, 109,
		"expected the idempotency wiring on at least 109 write call sites; found %d", scan.stamped)
}

// idempotencyStampScan carries the derived write universe and the credit the
// walk accumulates: one count per accounted write CALL.
type idempotencyStampScan struct {
	fset     *token.FileSet
	writeOps map[string]bool

	stamped int
}

// offences reports every write call in one facade function that leaves without
// an idempotency key, and credits the ones that do not.
//
// Each mention is judged in this order:
//
//  1. an operation on the deliberate no-key list is skipped outright, wherever it
//     is named. The exemption is a statement about the OPERATION — the tracer
//     dedups it on the body, or it moves no money — so it holds however the call
//     is reached, which is what lets the rule and reservation transitions hand
//     the operation to a helper as a function value;
//  2. every other mention must be the CALLEE of a call. A mention anywhere else
//     is a value this scan cannot follow to a request, so it is reported rather
//     than credited;
//  3. that call must carry a key — see carriesStamp.
func (s *idempotencyStampScan) offences(file string, fn *ast.FuncDecl) []string {
	mentions := writeMentions(fn.Body, s.writeOps)
	if len(mentions) == 0 {
		return nil
	}

	site := " (" + filepath.Base(file) + ":" + strconv.Itoa(s.fset.Position(fn.Pos()).Line) + ")"
	carriers := stampCarriers(fn, bodyVariables(fn))

	var problems []string

	for _, sel := range mentions {
		op, _ := generatedOperation(sel.Sel.Name)
		if _, exempt := unstampedWrites[op]; exempt {
			continue
		}

		where := op + " on line " + strconv.Itoa(s.fset.Position(sel.Pos()).Line)

		call, called := calleeCallOf(fn, sel)

		switch {
		case !called:
			problems = append(problems, funcLabel(fn)+site+" names "+where+
				" without calling it there, so no idempotency key can be verified for that mention")
		case !carriesStamp(call, carriers):
			problems = append(problems, funcLabel(fn)+site+" writes via "+where+
				" without an idempotency key")
		default:
			s.stamped++
		}
	}

	return problems
}

// writeOperationMethods are the HTTP methods that can change state. HEAD and GET
// are excluded; a repeated read is free.
var writeOperationMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// writeOperations reads the generated clients and returns the operations whose
// request builder issues a state-changing method.
func writeOperations(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			collectWriteOperations(t, file, ops)
		}
	}

	return ops
}

// collectWriteOperations records every request builder in one generated file
// that issues a write method.
func collectWriteOperations(t *testing.T, file *ast.File, ops map[string]bool) {
	t.Helper()

	operationsMatchingMethod(t, file, writeOperationMethods, ops)
}

// operationsMatchingMethod records every request builder in one generated file
// whose body issues one of the given methods. It is the one scan behind both
// generated-operation universes: the write set here, and the delete set in
// delete_seam_structural_test.go.
//
// Sharing it is the point. Both universes rest on the same reading of how
// oapi-codegen emits the method, and as two copies of the same loop the day that
// emission changes is the day one copy gets updated and the other goes silently
// narrow — a scan finding no operations still passes its own floor, because an
// empty universe has no offences in it.
func operationsMatchingMethod(t *testing.T, file *ast.File, methods, ops map[string]bool) {
	t.Helper()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv != nil {
			continue
		}

		op, ok := requestBuilderOperation(fn.Name.Name)
		if !ok {
			continue
		}

		if method, ok := requestBuilderMethod(t, fn); ok && methods[method] {
			ops[op] = true
		}
	}
}

// requestBuilderMethod returns the HTTP method one generated request builder
// issues, read from the http.NewRequest call in its body.
//
// # Old shape → new shape (oapi-codegen v2.4 → v2.7)
//
// v2.4 wrote the method as a bare string literal:
//
//	http.NewRequest("DELETE", queryURL.String(), nil)
//
// v2.7 writes the stdlib constant instead:
//
//	http.NewRequest(http.MethodDelete, queryURL.String(), nil)
//
// The operation SETS are unchanged across that rewrite — 225 request builders
// before and after, spelling 29 DELETE, 60 POST, 32 PATCH, 1 PUT, 89 GET and 14
// HEAD in both — so no floor in either dependent scan moves. Only the reading of
// the method did.
//
// # Why this is read from the AST now
//
// The previous reading regex-matched http\.NewRequest\("DELETE" in the builder's
// SOURCE TEXT, and both dependent scans carried the same caveat in their header:
// a builder naming its method any other way "would not match and its operation
// would drop out of the universe silently". v2.7 is precisely that case, and it
// is precisely what happened — three structural scans came back with empty
// universes on the upgrade.
//
// So the residual is closed rather than restated for a third time. The first
// argument of the call is read, both spellings that denote a method constant are
// accepted, and anything else is a hard FAILURE rather than a skip. A future
// emission change therefore cannot narrow these universes without saying so.
func requestBuilderMethod(t *testing.T, fn *ast.FuncDecl) (string, bool) {
	t.Helper()

	var (
		method string
		found  bool
	)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !isNewRequestCall(call) {
			return true
		}

		method, found = httpMethodOf(call.Args[0])

		require.True(t, found,
			"%s issues http.NewRequest with a method this scan cannot read; both generated-operation "+
				"universes are derived from it, so an unreadable method would silently narrow them "+
				"instead of failing here", fn.Name.Name)

		return false
	})

	return method, found
}

// isNewRequestCall reports whether a call is net/http's NewRequest.
func isNewRequestCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "NewRequest" {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)

	return ok && pkg.Name == "http"
}

// httpMethodOf resolves the method argument of an http.NewRequest call to an
// uppercase method name, in the two spellings that denote one: a string literal
// ("DELETE") and the stdlib constant (http.MethodDelete).
//
// Nothing else is accepted. A method reached through a local variable, a
// generated helper or a computed expression is reported as unreadable by the
// caller, because a universe derived from it would be narrow rather than wrong —
// and a narrow universe passes every assertion built on it.
func httpMethodOf(arg ast.Expr) (string, bool) {
	switch node := arg.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}

		method, err := strconv.Unquote(node.Value)

		return method, err == nil && method != ""
	case *ast.SelectorExpr:
		pkg, ok := node.X.(*ast.Ident)
		if !ok || pkg.Name != "http" {
			return "", false
		}

		name, ok := strings.CutPrefix(node.Sel.Name, "Method")

		return strings.ToUpper(name), ok && name != ""
	}

	return "", false
}

// writeMentions returns the mentions of a generated write operation inside a
// node — one entry per OCCURRENCE, not one per operation.
//
// Returning NODES rather than names is what lets the caller judge each mention
// where it sits. A set keyed by operation name collapses every distinct mention
// of one operation into a single entry, which is how one stamped write used to
// launder every other mention of that name in the same function.
func writeMentions(node ast.Node, writeOps map[string]bool) []*ast.SelectorExpr {
	var mentions []*ast.SelectorExpr

	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if op, ok := generatedOperation(sel.Sel.Name); ok && writeOps[op] {
			mentions = append(mentions, sel)
		}

		return true
	})

	return mentions
}

// carriesStamp reports whether ONE write call resolves a key.
//
// Two positions count, and both are properties of THIS call rather than of the
// function around it:
//
//   - a stamp call is a direct argument of the write call. That is the ledger and
//     tracer shape, where the resolved editors are the variadic tail:
//     f.ledger.DeleteAsset(ctx, ..., idempotencyEditors(ctx, gate)...); or
//   - the write call names a body variable that a stamp call touched — see
//     stampCarriers. That is the transactions shape, where the key lands on a
//     params struct (applyIdempotency(&params.XIdempotency, ...)) or comes back
//     out of prepareCreate, and the struct is then handed to the generated method.
//
// # What this matcher can and cannot see
//
// It is source-structural. There is no type information and no dataflow: it
// matches the identifiers a stamp call touches against the identifiers a write
// call mentions, and it cannot prove that the variable it matched is the one
// carrying the key. Its two known residuals, both in the loosening direction:
//
//   - a stamp call's ARGUMENTS credit every body variable they name, so a
//     variable passed to a stamp for some other reason would credit a write that
//     also names it. Live, the only such variables are the params structs the
//     stamp writes into, which is the intended link;
//   - an assignment FROM a stamp preparer credits every body variable on its left,
//     err included, because the carrier and the error come back together
//     (scoped, params, err := prepareCreate(...)). No generated client method takes
//     an error, so nothing live can be credited through that name.
//
// Parameters, the receiver and body constants are excluded from both sides by
// bodyVariables, which is what stops ctx — shared by every call in the function
// by construction — from linking anything to anything.
func carriesStamp(call *ast.CallExpr, carriers map[string]bool) bool {
	for _, arg := range call.Args {
		if inner, ok := ast.Unparen(arg).(*ast.CallExpr); ok && isStampCall(inner) {
			return true
		}
	}

	for name := range identifiersIn(call.Args) {
		if carriers[name] {
			return true
		}
	}

	return false
}

// stampCarriers returns the body variables a stamp call touches: the ones it
// RECEIVES, and the ones it is ASSIGNED TO.
//
// Both directions are needed because the two live shapes point opposite ways.
// applyIdempotency writes the key THROUGH its argument, so the carrier is on the
// way in; prepareCreate returns the resolved params, so the carrier is on the way
// out.
func stampCarriers(fn *ast.FuncDecl, locals map[string]bool) map[string]bool {
	carriers := map[string]bool{}

	collectStampArguments(fn, locals, carriers)
	collectStampResults(fn, locals, carriers)

	return carriers
}

// collectStampArguments records the body variables a stamp call RECEIVES, which
// is the direction applyIdempotency writes in.
func collectStampArguments(fn *ast.FuncDecl, locals, carriers map[string]bool) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isStampCall(call) {
			return true
		}

		for name := range identifiersIn(call.Args) {
			if locals[name] {
				carriers[name] = true
			}
		}

		return true
	})
}

// collectStampResults records the body variables a stamp call is ASSIGNED TO,
// which is the direction prepareCreate returns in.
func collectStampResults(fn *ast.FuncDecl, locals, carriers map[string]bool) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || !assignsStampResult(assign) {
			return true
		}

		for _, lhs := range assign.Lhs {
			if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok && locals[ident.Name] {
				carriers[ident.Name] = true
			}
		}

		return true
	})
}

// isStampCall reports whether a call puts a key on the wire or resolves one on a
// caller's behalf.
func isStampCall(call *ast.CallExpr) bool {
	name, ok := calleeName(call.Fun)

	return ok && (idempotencyHelpers[name] || stampPreparers[name])
}

// assignsStampResult reports whether the assignment binds the results of a stamp
// call.
func assignsStampResult(assign *ast.AssignStmt) bool {
	for _, rhs := range assign.Rhs {
		if call, ok := ast.Unparen(rhs).(*ast.CallExpr); ok && isStampCall(call) {
			return true
		}
	}

	return false
}
