package entities

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// transitionHelpers are the package-local helpers that take a generated client
// method as a function VALUE and call requirePathIDs on the caller's behalf. A
// facade method that delegates to one of these never names the generated
// operation in a call of its own, so the structural check credits the
// delegation instead.
//
// The list is deliberately literal rather than derived from a call graph. A
// call graph would credit any local helper that happens to guard somewhere,
// which turns "this method is guarded" into "something this method touches
// guards something". Adding a fourth helper here should be a decision someone
// makes on purpose.
var transitionHelpers = map[string]bool{
	"ruleTransition":        true,
	"limitTransition":       true,
	"reservationTransition": true,
}

// TestEveryPathParameterOperationIsGuarded is the structural invariant behind
// the empty/dot-segment path id guard.
//
// The table-driven guard tests sample one call per facade family, which proves
// the family is wired up but says nothing about the other methods in it:
// deleting the guard from a method no row happens to name leaves the suite
// green. This test closes that gap by checking the property directly, on every
// method at once.
//
// The rule it enforces: if a facade function calls a generated client operation
// whose request builder formats a value into the URL PATH, then EVERY value it
// forwards into that path must be one the function handed to requirePathIDs.
//
// # Why identity, and not the presence of the call
//
// An earlier version asked only whether requirePathIDs appeared somewhere in the
// function. That accepted a call naming NO pairs at all — requirePathIDs(op)
// returns nil on an empty list — so a facade could satisfy the scan with a guard
// that checked nothing, and every by-id read behind it would be back to the
// silent-zero and scope-escalation defects two fix rounds were spent closing.
// There were no live instances; it was a hole in the proof rather than a defect
// in the product, which is exactly the kind of hole that stops being theoretical
// the first time someone edits a facade to make a test stop complaining.
//
// The check now compares expressions. The path ARGUMENT POSITIONS come from the
// generated code — the request builder says which of its parameters it styles
// into the path, and the client method's signature says where those parameters
// sit in the argument list — so a facade forwarding orgID into a path segment
// must have named orgID to the guard. Guarding a different variable, or none,
// fails.
//
// # Why PLACEMENT and EFFECT, and not identity alone
//
// Comparing expressions is still not enough by itself. Successive reviews wrote
// compiling facades that sent id=".." to the wire with a nil error and passed
// the identity check anyway, because the check asked where the guard's NAME
// appeared and never what the guard did. Each of these is closed, and each was
// re-run against the scan before and after:
//
//   - the guard placed AFTER the generated call, which validates a request that
//     has already left — closed by requiring the guard to precede the call;
//   - the guard nested inside an if, so the path it guards has a way around it —
//     closed by requiring statement depth 1 and unconditional execution;
//   - the generated call hoisted into a local first (get := f.ledger.GetX),
//     which made the call target a plain identifier, dropped the direct-call
//     count to zero and fell through to the weakest branch. That is the same
//     hoist that defeated the sibling delete-seam scan in Epic 3 and was closed
//     there by matching the operation rather than the call shape; directPathCalls
//     now follows the binding;
//   - the vacuous requirePathIDs(operation) naming no pairs, which returns nil on
//     an empty list — closed by requiring real pairs on every branch that credits;
//   - the DEFANGED guard, which runs, names every pair, and discards the verdict
//     (`if err := requirePathIDs(...); err != nil { _ = err }`, `_ = require...`,
//     `err := require...` never checked, `defer require...`) — closed by
//     guardCallActedOn, which accepts one spelling and pins every part of it;
//   - the DEFORMED CONDITION, which keeps that spelling's initialiser and body
//     and rewrites the test between them (`err == nil`, `err != nil && f.strict`,
//     `ctx.Err() != nil`) so the return is unreachable, or reachable only
//     sometimes, while the guard still reads as one — closed by pinning the
//     condition too, and with it the value the body returns;
//   - the TOKEN guard at a delegation site, naming a value that never becomes a
//     path segment while the id that does is forwarded to a helper outside
//     transitionHelpers. The branch that tried to judge this AT THE CALL SITE is
//     deleted: it had zero live users and four ways to be talked into crediting
//     an unguarded id. Delegation to a helper the scan does not know is now
//     REPORTED — see unknownDelegation.
//
// # Known ceiling, and it is deliberate
//
// Three things remain out of reach, and faking them with heuristics would be
// worse than saying so:
//
//   - a guarded variable REASSIGNED between the guard and the call —
//     requirePathIDs(op, "id", id), then id = "..", then f.ledger.GetX(ctx, id);
//   - an inner scope SHADOWING a guarded name, so the identifier the call
//     forwards is a different variable wearing the same spelling;
//   - REFLECTION — a client method resolved by name at runtime, which is
//     invisible to AST matching. This is the ceiling all three structural scans
//     in this package share; nothing here does it, and none of the three pretends
//     to cover it.
//
// The first two are invisible to a scan that compares source text, and closing
// either one honestly needs type-checked SSA: resolving every identifier to its
// definition and proving no assignment reaches the call. That is a different
// tool, not a stricter match.
//
// # Accepted strictness, so nobody "fixes" it
//
// Three false positives are deliberate, and each is the price of a match that
// cannot be talked into accepting a hostile input:
//
//   - A DERIVED value is refused even when the derivation is harmless: guarding
//     orgID and forwarding strings.ToLower(orgID) is reported as unguarded. The
//     only way to accept it is to loosen the match until an expression MENTIONING
//     a guarded name counts, and that same loosening accepts orgID + "/../" —
//     precisely the input two fix rounds were spent rejecting. A facade that
//     needs to normalise an id should assign the normalised value to a name,
//     guard that name, and forward that name.
//   - A guard placed after a generated call that sits inside a DEFERRED CLOSURE
//     is reported, even though the guard genuinely runs first: the deferred body
//     executes when the function returns. Accepting it means reasoning about
//     execution order rather than about source position, which is the same class
//     of reasoning that let a guard below a call pass in the first place. The
//     shape carries no innocent meaning in a facade — a generated call belongs in
//     the body, not in a defer.
//   - A guard whose body returns a WRAPPED error is reported, even though it
//     genuinely propagates: `return nil, fmt.Errorf("%s: %w", operation, err)`
//     refuses, because the credited spelling requires the guard's own identifier
//     among the return's results. Accepting an expression that merely MENTIONS
//     the error means accepting one that discards it, and `_ = err;
//     return nil, nil` is a live shape someone writes to quiet a test. No live
//     site wraps — all 202 return the bare identifier — so this costs nothing
//     today, and requirePathIDs already builds an *errors.Error carrying the
//     operation label, which is what a wrapper would be adding. If a facade ever
//     needs to wrap, widening rule 4 to "the identifier appears WITHIN one of the
//     results" is the honest change, and it belongs here rather than in a
//     //nolint at the site.
//
// # This scan depends on its sibling
//
// The coverage claimed here is conditional on TestNoFacadeCallsAGeneratedParser
// holding. Path argument POSITIONS are resolved from the generated raw *Client
// signatures, so a call reached through any other spelling — a parser method, a
// Parse*Resp free function — has no resolvable positions. Such a call is now
// REPORTED rather than credited (see unguardedPathArguments), which is what
// keeps an unknown spelling from buying a free pass here; but the scan that
// keeps those spellings out of the package in the first place is the sibling's.
//
// Every input is read out of the source at test time rather than from a
// checked-in list, so a newly generated operation or a newly written facade
// method is covered the moment it lands, without anyone remembering to add it.
func TestEveryPathParameterOperationIsGuarded(t *testing.T) {
	fset := token.NewFileSet()

	pathOps := operationsWithPathParameters(t, fset)
	require.NotEmpty(t, pathOps, "found no generated operations with path parameters; the scan is broken, not the code")

	scan := pathGuardScan{
		fset:         fset,
		pathOps:      pathOps,
		pathArgs:     pathArgumentNames(t, fset),
		methodParams: clientMethodParameters(t, fset),
	}

	var unguarded []string

	guarded := 0
	helperGuards := map[string]bool{}

	for name, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			problem, credited := scan.classify(name, decl, helperGuards)
			if problem != "" {
				unguarded = append(unguarded, problem)
			}

			guarded += credited
		}
	}

	// A transition helper takes the generated method as a PARAMETER, so its own
	// body never names the operation and the loop above cannot see it. Its
	// callers are credited purely on the promise that it guards, so that promise
	// has to be checked here or deleting the guard from the helper silently
	// unguards every method that delegates to it.
	for name := range transitionHelpers {
		require.True(t, helperGuards[name],
			"%s is credited as a path-id guard for the facade methods that delegate to it, so it must "+
				"hand every id it forwards to the generated call to requirePathIDs; found=%v, guards=%v",
			name, helperGuards[name], helperGuards)
	}

	sort.Strings(unguarded)

	require.Empty(t, unguarded,
		"every facade function that formats a caller value into a URL path must reject bad path ids "+
			"locally, via requirePathIDs or a transition helper:\n  %s",
		strings.Join(unguarded, "\n  "))

	t.Logf("guard verified on %d facade functions across %d generated operations with path parameters",
		guarded, len(pathOps))

	// A floor, so the scan silently matching nothing cannot read as success.
	require.Greater(t, guarded, 100,
		"expected the guard on more than 100 facade functions; found %d, so the scan stopped seeing them", guarded)
}

// pathGuardScan carries the three derived inputs the per-declaration check
// needs, so the check is one method with one argument rather than a loop body
// nested inside the test function. Flattening it that way is also what keeps the
// test under the cognitive-complexity limit, which the COLD linter has caught in
// this package once per fix round.
type pathGuardScan struct {
	fset         *token.FileSet
	pathOps      map[string]bool
	pathArgs     map[string]map[string]bool
	methodParams map[string][]string
}

// classify judges one top-level declaration, returning the offence to report (or
// "") and 1 when the declaration is a facade function whose path ids are
// verifiably guarded.
//
// It also records a transition helper's own verdict into helperGuards, because
// the helper is the one shape whose guard cannot be judged from the call sites
// that depend on it.
func (s pathGuardScan) classify(path string, decl ast.Decl, helperGuards map[string]bool) (string, int) {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return s.packageScopeBinding(path, decl), 0
	}

	if transitionHelpers[fn.Name.Name] {
		helperGuards[fn.Name.Name] = helperGuardsWhatItForwards(fn, s.pathOps)
	}

	ops := pathOperationsNamedBy(fn.Body, s.pathOps)
	if len(ops) == 0 {
		return "", 0
	}

	site := " (declared at " + filepath.Base(path) + ":" +
		strconv.Itoa(s.fset.Position(fn.Pos()).Line) + ")"

	if calls := directPathCalls(fn, s.pathOps); len(calls) > 0 {
		missing := unguardedPathArguments(fn, calls, s.pathArgs, s.methodParams)
		if len(missing) > 0 {
			return funcLabel(fn) + " forwards " + strings.Join(missing, ", ") +
				" into a URL path without handing it to requirePathIDs" + site, 0
		}

		return "", 1
	}

	// The operation is NAMED but not called: the function hands it to a
	// transition helper as a function value, so there are no call arguments here
	// to compare. The helper's own guard is checked by the caller.
	if delegatesToTransitionHelper(fn) {
		return "", 1
	}

	return unknownDelegation(fn, ops, site), 0
}

// unknownDelegation reports the last shape: the function NAMES a path-parameter
// operation, does not call it, and does not delegate to a known transition
// helper — so it hands the operation as a function value to some other
// package-local helper, and there is no helper on record whose interior can be
// checked.
//
// # Why this is REPORTED rather than judged in place
//
// An earlier version tried to judge it at the call site, by applying the rule
// helperGuardsWhatItForwards applies inside a KNOWN helper: every value handed
// over alongside the operation had to be one the guard received. That branch was
// measured against the live tree and had ZERO users — all 209 credited functions
// go through the direct-call branch (199) or a known transition helper (10) —
// while carrying four independent ways to be talked into crediting an unguarded
// id:
//
//   - the operation HOISTED into a local first, so the delegation call carries a
//     plain identifier and the branch that looked for a selector saw no
//     delegation at all — the same hoist that defeated the direct-call branch and
//     the sibling delete-seam scan before it;
//   - no ORDERING requirement: the guard was collected up to the function's
//     closing brace, so a guard written AFTER the delegation credited it, which
//     is the guard-below-the-call shape this scan closed everywhere else;
//   - the variadic-tail exclusion, sound for a GENERATED method (oapi-codegen
//     puts request editors there, never a path parameter) and unsound the moment
//     it was applied to a LOCAL helper whose tail the author writes — `ids...`
//     carried the id straight through;
//   - the context exclusion is by NAME, so a value spelled like the context
//     parameter was dropped without ever being compared.
//
// Closing four escapes in a branch nothing uses buys nothing and has to be
// maintained. Unknown is therefore FLAGGED, which is the rule the sibling seam
// scan already follows for a spelling it cannot resolve, and which
// unguardedPathArguments follows for an operation reached through an
// unrecognised generated spelling. All four escapes close by construction, and
// the scan gets smaller.
//
// The cost is one deliberate false positive, and it is the friction
// transitionHelpers exists to create: a facade delegating to a NEW local helper
// fails until someone adds that helper to transitionHelpers, where
// helperGuardsWhatItForwards then checks its interior. The list is literal on
// purpose — "adding a fourth helper here should be a decision someone makes on
// purpose" — and this is what makes that decision unavoidable rather than
// optional.
func unknownDelegation(fn *ast.FuncDecl, ops []string, site string) string {
	return funcLabel(fn) + " hands " + strings.Join(ops, ", ") +
		" to a local helper that is not in transitionHelpers, so no guard can be verified" +
		" — add the helper to that list, where its interior is checked" + site
}

// forwardedValues returns the arguments of one call that could carry a caller's
// value into a URL path, as SOURCE TEXT — the same spelling guardedValues
// records, so the two are compared as written.
//
// Its one caller is helperGuardsWhatItForwards, so the call it reads is always a
// call to a KNOWN transition helper's function-typed parameter: the generated
// method itself. That is what makes the exclusions below sound — see
// unknownDelegation for the branch that applied the same exclusions to an
// arbitrary local helper, where two of them stopped being true, and for why that
// branch is gone.
//
// # Expression text, not identifiers
//
// An earlier version collected `*ast.Ident` arguments only and silently dropped
// every other shape, on a justification that was simply false: it claimed the
// direct-call comparison also accepted identifiers only, when that comparison
// has always compared types.ExprString of whatever expression sat in a path
// position. A selector (`ids.ID`) and an index (`ids[0]`) are not exotic; they
// are what someone writes the day a helper carries its ids in a struct or a
// slice. So every argument is compared, whatever its AST shape, and an unmatched
// one is REPORTED. The strictness that follows is the same strictness the
// direct-call branch already carries: guarding `ids.ID` and forwarding `ids.ID`
// passes, guarding `ids` and forwarding `ids.ID` does not.
//
// # What is excluded, and why each one cannot become a path segment
//
//   - fn's CONTEXT parameter, by name rather than by position. The positional
//     "the context is argument 0" assumption is true of every helper today and
//     costs nothing to drop. By NAME is a weaker rule than by TYPE, and the
//     residual is recorded on unknownDelegation; it survives here because the
//     three helpers this now runs on are a literal list a human curates.
//   - a LITERAL, and a local `const operation = "Segments.Get"`. Both are fixed
//     at compile time; the const is the error label every facade declares, and it
//     cannot carry ".." in from a caller. Requiring it to be guarded would be a
//     false positive on the correctly-written form of the very shape this check
//     exists to catch.
//   - the GENERATED OPERATION, when a helper forwards one (`f.tracer.ActivateRule`).
//     It is a function value, matched by resolving the selector against pathOps,
//     which is why `ids.ID` — a selector too — is not excluded with it.
//   - the VARIADIC SPREAD at the tail (`idempotencyEditorsTracer(ctx, false)...`,
//     which all three live helpers forward). oapi-codegen never puts a path
//     parameter in a GENERATED method's variadic tail; that tail is the request
//     editors, always — and a generated method is the only thing the surviving
//     caller reads, which is exactly what makes this exclusion sound here and
//     made it unsound in the deleted branch.
func forwardedValues(fn *ast.FuncDecl, call *ast.CallExpr, pathOps map[string]bool) []string {
	ctxName := contextParameter(fn)
	constants := localStringConstants(fn)

	var values []string

	for i, arg := range call.Args {
		if isVariadicSpread(call, i) || isFixedArgument(arg, ctxName, constants, pathOps) {
			continue
		}

		values = append(values, types.ExprString(arg))
	}

	return values
}

// isVariadicSpread reports whether the argument at index i is the `xs...` tail of
// the call.
func isVariadicSpread(call *ast.CallExpr, i int) bool {
	return call.Ellipsis.IsValid() && i == len(call.Args)-1
}

// isFixedArgument reports whether one argument is fixed at compile time, is the
// context, or is the generated operation being delegated — none of which a
// caller can turn into a path segment.
func isFixedArgument(arg ast.Expr, ctxName string, constants, pathOps map[string]bool) bool {
	switch node := arg.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return node.Name == ctxName || constants[node.Name]
	case *ast.SelectorExpr:
		op, ok := generatedOperation(node.Sel.Name)

		return ok && pathOps[op]
	}

	return false
}

// localStringConstants returns the names fn binds to a string literal in a const
// declaration of its own.
func localStringConstants(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}

		for _, spec := range decl.Specs {
			collectStringConstantNames(spec, names)
		}

		return true
	})

	return names
}

// collectStringConstantNames records the names in one const spec bound to a
// string literal.
func collectStringConstantNames(spec ast.Spec, names map[string]bool) {
	value, ok := spec.(*ast.ValueSpec)
	if !ok {
		return
	}

	for i, name := range value.Names {
		if i >= len(value.Values) {
			continue
		}

		if lit, ok := value.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			names[name.Name] = true
		}
	}
}

// contextParameter returns the name of fn's context.Context parameter, or "".
func contextParameter(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}

	for _, field := range fn.Type.Params.List {
		sel, ok := field.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Context" || len(field.Names) == 0 {
			continue
		}

		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "context" {
			return field.Names[0].Name
		}
	}

	return ""
}

// delegatesToTransitionHelper reports whether fn hands its generated operation to
// one of the transition helpers, which guard on their caller's behalf. The
// helpers' own guards are checked separately, in the test body, because a helper
// is credited by every method that delegates to it.
func delegatesToTransitionHelper(fn *ast.FuncDecl) bool {
	return callsAny(fn, func(name string) bool { return transitionHelpers[name] })
}

// packageScopeBinding reports a path-parameter operation bound OUTSIDE any
// function body.
//
// A package-level var, const or type that binds one leaves every caller naming a
// bare identifier, which no scan of function bodies can attribute back to the
// operation — so the guard requirement evaporates silently. See
// packageScopeEscape (delete_seam_structural_test.go) for the two shapes this
// was proven against and for the reflection ceiling both scans share.
func (s pathGuardScan) packageScopeBinding(path string, decl ast.Decl) string {
	ops := pathOperationsNamedBy(decl, s.pathOps)
	if len(ops) == 0 {
		return ""
	}

	return "the package-level declaration in " + filepath.Base(path) + ":" +
		strconv.Itoa(s.fset.Position(decl.Pos()).Line) + " binds " + strings.Join(ops, ", ") +
		", which no caller can then be checked for a guard"
}

// pathCall is one INVOCATION of a generated path-parameter operation inside a
// facade: the call itself, plus the exact generated spelling it reaches.
//
// The spelling travels with the call because the argument LIST differs between
// an operation's spellings, and because a call reached through a hoisted local
// names the operation somewhere other than at the call site.
type pathCall struct {
	call   *ast.CallExpr
	method string
}

// directPathCalls returns the calls in fn that INVOKE a generated
// path-parameter operation, as opposed to merely naming one.
//
// The distinction is what decides which check applies: an invoked operation has
// argument expressions to compare against the guard, while an operation handed
// to a transition helper as a function value has none.
func directPathCalls(fn *ast.FuncDecl, pathOps map[string]bool) []pathCall {
	hoisted := hoistedOperations(fn, pathOps)

	var calls []pathCall

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if method, ok := calledOperationSpelling(call.Fun, pathOps, hoisted); ok {
			calls = append(calls, pathCall{call: call, method: method})
		}

		return true
	})

	return calls
}

// calledOperationSpelling returns the generated spelling a call target reaches,
// whether the operation is named at the call site (f.ledger.GetAccountByID(...))
// or bound to a local first.
func calledOperationSpelling(fun ast.Expr, pathOps map[string]bool, hoisted map[string]string) (string, bool) {
	switch target := fun.(type) {
	case *ast.SelectorExpr:
		if op, ok := generatedOperation(target.Sel.Name); ok && pathOps[op] {
			return target.Sel.Name, true
		}
	case *ast.Ident:
		if method, ok := hoisted[target.Name]; ok {
			return method, true
		}
	}

	return "", false
}

// hoistedOperations returns the local identifiers fn binds to a generated
// path-parameter operation's method VALUE, mapped to the spelling each carries.
//
//	get := f.ledger.GetAccountByID
//	resp, err := get(ctx, orgID, ledgerID, id)
//
// leaves the call target a plain identifier, so a scan requiring a selector on
// call.Fun counts zero calls and falls through to the weaker branch in classify.
// That is the hoist the sibling delete-seam scan was defeated by in Epic 3 and
// closed against; this is the same closure, ported. Binding a generated
// operation to a local carries no innocent meaning in a facade, so the binding
// is simply followed.
func hoistedOperations(fn *ast.FuncDecl, pathOps map[string]bool) map[string]string {
	bound := map[string]string{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			bindOperations(bound, node.Lhs, node.Rhs, pathOps)
		case *ast.ValueSpec:
			bindOperations(bound, identExprs(node.Names), node.Values, pathOps)
		}

		return true
	})

	return bound
}

// bindOperations records every name on the left that is bound to a generated
// path-parameter operation on the right.
func bindOperations(bound map[string]string, lhs, rhs []ast.Expr, pathOps map[string]bool) {
	if len(lhs) != len(rhs) {
		return
	}

	for i, value := range rhs {
		sel, ok := value.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		op, ok := generatedOperation(sel.Sel.Name)
		if !ok || !pathOps[op] {
			continue
		}

		if name, ok := lhs[i].(*ast.Ident); ok {
			bound[name.Name] = sel.Sel.Name
		}
	}
}

// identExprs widens a var declaration's names to expressions, so one binding
// walker serves both `x := expr` and `var x = expr`.
func identExprs(names []*ast.Ident) []ast.Expr {
	out := make([]ast.Expr, 0, len(names))
	for _, name := range names {
		out = append(out, name)
	}

	return out
}

// unguardedPathArguments returns the expressions fn forwards into a URL path
// that it did not hand to requirePathIDs.
//
// This is the identity check. For each generated operation the function calls,
// it takes the argument positions the generated client formats into the path and
// compares those expressions, as source text, against the values the guard
// received. An empty result means every path segment the caller can influence
// was checked before the request was built.
//
// # An unresolvable spelling is a FINDING, not a defensive branch
//
// Argument positions come from clientMethodParameters, which indexes methods on
// the raw *Client only. So a call reached through any OTHER spelling — the
// *ClientWithResponses parser method, the ParseOpResp free function — resolves
// to no positions at all. Returning early there is what coupled this scan to its
// sibling: an unknown spelling yielded no path arguments, nothing was missing,
// and the call was CREDITED as guarded for free. It was measured, not reasoned
// about — a facade calling GetSegmentByIDWithResponse with three
// caller-controlled ids and no guard whatsoever passed this scan, and was caught
// only by TestNoFacadeCallsAGeneratedParser, one test away.
//
// Unknown-but-generated is therefore reported. The two scans still lean on each
// other by design and each says so, but neither one hands out credit for a call
// it could not read. Do not "simplify" this branch back into a skip.
func unguardedPathArguments(
	fn *ast.FuncDecl,
	calls []pathCall,
	pathArgs map[string]map[string]bool,
	methodParams map[string][]string,
) []string {
	checked := guardedValues(fn, earliestCall(calls))

	var missing []string

	for _, pc := range calls {
		op, _ := generatedOperation(pc.method)

		names, wanted := methodParams[pc.method], pathArgs[op]
		if len(names) == 0 || len(wanted) == 0 {
			missing = append(missing, "every path argument of "+op+
				" — reached through "+pc.method+", a spelling with no raw *Client signature, so this "+
				"scan cannot resolve which arguments become path segments —")

			continue
		}

		for _, arg := range pathArgumentsOf(pc.call, names, wanted) {
			if !checked[arg] {
				missing = append(missing, arg+" (into "+op+")")
			}
		}
	}

	sort.Strings(missing)

	return missing
}

// earliestCall returns the position of the first generated call in source order,
// which is the cutoff a guard has to beat.
func earliestCall(calls []pathCall) token.Pos {
	earliest := calls[0].call.Pos()

	for _, pc := range calls[1:] {
		if pc.call.Pos() < earliest {
			earliest = pc.call.Pos()
		}
	}

	return earliest
}

// pathArgumentsOf returns the argument expressions of one call that the
// generated client styles into the URL path.
//
// names are the ordered parameter names of the exact spelling at the call site
// (GetAssetByID, CreateAssetWithBody, ...), because the argument LIST differs
// between an operation's spellings; wanted are the parameter names its request
// builder styles into the path.
func pathArgumentsOf(call *ast.CallExpr, names []string, wanted map[string]bool) []string {
	var args []string

	for i, name := range names {
		if !wanted[name] || i >= len(call.Args) {
			continue
		}

		args = append(args, types.ExprString(call.Args[i]))
	}

	return args
}

// guardedValues returns the VALUE half of every requirePathIDs pair fn hands to
// the guard before position before, as source text.
// requirePathIDs(operation, "orgID", orgID, "id", id) contributes orgID and id —
// the names are labels for the error message and prove nothing about what was
// checked.
//
// Three requirements separate a guard from something that merely looks like one,
// and each came from a compiling facade that passed without it:
//
//   - The statement carrying the guard must be at depth 1 of the function body
//     and the call must run UNCONDITIONALLY within it. The idiomatic
//     `if err := requirePathIDs(...); err != nil { return nil, err }` qualifies,
//     because the call sits in the if's initialiser, which always executes; a
//     guard nested inside another if, a loop, a switch case or a closure does
//     not, because the call it guards has a way around it.
//   - It must appear BEFORE the generated call. A guard below the call returns a
//     validation error about a request that has already reached the wire.
//   - Its ERROR must be acted on — see guardCallActedOn. Running the guard and
//     dropping its verdict is not a guard, and it was the shape that passed every
//     other requirement here.
func guardedValues(fn *ast.FuncDecl, before token.Pos) map[string]bool {
	values := map[string]bool{}

	for _, stmt := range fn.Body.List {
		if stmt.Pos() >= before {
			break
		}

		call, ok := guardCallActedOn(stmt, "requirePathIDs")
		if !ok {
			continue
		}

		// args[0] is the operation label; the pairs start at 1, values at 2.
		for i := 2; i < len(call.Args); i += 2 {
			values[types.ExprString(call.Args[i])] = true
		}
	}

	return values
}

// guardCallActedOn returns the guard call in one statement whose error the
// statement actually ACTS ON, which is a strictly narrower question than whether
// the call runs.
//
// Every earlier version of this scan asked only whether the guard EXECUTED. That
// accepted a facade which ran the guard, named every pair correctly, and then
// threw the verdict away:
//
//	if err := requirePathIDs(op, "orgID", orgID, "id", id); err != nil {
//		_ = err
//	}
//	resp, err := f.ledger.GetSegmentByID(ctx, orgID, ledgerID, id)
//
// That compiles, passes a cold golangci-lint run, sends id=".." to the wire with
// a nil error, and is the diff someone writes at the end of a long day to make a
// test stop complaining. It reopens the scope escalation two fix rounds closed —
// ".." pops a path segment, so deleting a ledger deletes the organization.
//
// # One spelling, and ALL of it
//
// So exactly one spelling counts, the one all 202 live call sites already use:
//
//	if <err> := requirePathIDs(op, ...pairs...); <err> != nil {
//		return ..., <err>
//	}
//
// An earlier version of this function pinned two of that shape's four parts —
// the initialiser and the presence of a return — and left the CONDITION and the
// RETURNED VALUE unspecified. Three deformations walked straight through the
// gap, each keeping the accepted initialiser and an accepted return:
//
//	if err := requirePathIDs(op, ...); err == nil { return nil, err }
//	if err := requirePathIDs(op, ...); err != nil && f.strict { return nil, err }
//	if err := requirePathIDs(op, ...); ctx.Err() != nil { return nil, err }
//
// The first returns only when the ids are FINE, so a bad id falls through to the
// wire. The second returns only when some other flag agrees. The third tests a
// value that has nothing to do with the ids. All three send id=".." with a nil
// error, and all three read, at a glance, exactly like a guard. A fourth kept
// the condition and threw the verdict away in the return itself —
// `{ _ = err; return nil, nil }` — which stops the request and hands the caller
// a nil error for a rejected id, the silent-zero defect wearing a guard's
// clothes.
//
// # Naming a part is not constraining it
//
// The round that pinned the condition also NAMED the body — "the Body returns
// directly" — and then wrote a matcher that looked for A return among the body's
// statements and let anything precede it. Three more deformations kept the
// initialiser, the condition and a qualifying return, and put a rejected id
// either on the wire or past the caller with a nil error; they are set out on
// returnsOnlyIdent, which is where that part is now actually constrained.
//
// So the lesson has two halves. The complement of a shape is only closed when
// EVERY part of the shape is pinned — and a part is pinned only when a
// deformation violating THAT PART ALONE is refused. The method that proves it is
// mechanical: after writing an N-part matcher, write N single-part deformations,
// one per part, each violating exactly one, and confirm each is flagged. Round 3
// probed the condition three ways and the return's results once, and never
// probed the body in isolation, so the body read as closed because the parts
// around it were.
//
// The four parts, and a guard is credited only when all four hold:
//
//  1. the Init binds exactly one identifier to a DIRECT call to the guard —
//     `err := requirePathIDs(...)`, not `err := wrap(requirePathIDs(...))`,
//     whose result says nothing about the guard's verdict;
//  2. the Cond is exactly `<that identifier> != nil` — nothing AND-ed or OR-ed
//     onto it, no other operator, no other operand;
//  3. the Body is EXACTLY ONE statement and that statement is a return, so
//     nothing runs between the rejection and the return; and
//  4. that return carries the guard's own error among its results.
//
// Everything else drops out without being named. `_ = requirePathIDs(...)`,
// `err := requirePathIDs(...)` never checked, and `defer requirePathIDs(...)`
// are not an if statement with this initialiser; a guard in the condition rather
// than the initialiser (`if requirePathIDs(...) != nil`) binds no identifier for
// the return to carry; a guard inside a nested block, a loop, or a closure is
// not a statement of the function body at all, which is where guardedValues
// looks.
//
// The return must be the body's ONLY statement, and must carry the bare
// identifier. Both are stricter than the language requires, and both stay strict
// on purpose: every one of the 202 live sites is the flat one-statement spelling
// returning the bare err (161 `return nil, err`, 29 `return err`, 12
// `return 0, err`, counted against the live tree before each rule landed), so
// strictness costs nothing today. Loosening "returns the error" to "returns something mentioning the
// error" is the same loosening that would accept orgID + "/../" on the argument
// side. A facade that genuinely needs to wrap the guard's error is the one
// deliberate false positive this adds; see the accepted-strictness list above.
func guardCallActedOn(stmt ast.Stmt, name string) (*ast.CallExpr, bool) {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok {
		return nil, false
	}

	errName, call, ok := guardAssignment(ifStmt.Init, name)
	if !ok || !comparesNotNil(ifStmt.Cond, errName) || !returnsOnlyIdent(ifStmt.Body, errName) {
		return nil, false
	}

	return call, true
}

// guardAssignment returns the single identifier a statement binds to a DIRECT
// call to name, and that call.
//
// Direct is the load-bearing word: an interposed call
// (`err := wrap(requirePathIDs(...))`) yields an error that is no longer the
// guard's verdict, and wrap is free to return nil for a rejected id.
func guardAssignment(stmt ast.Stmt, name string) (string, *ast.CallExpr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", nil, false
	}

	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isIdent(call.Fun, name) {
		return "", nil, false
	}

	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return "", nil, false
	}

	return lhs.Name, call, true
}

// comparesNotNil reports whether cond is exactly `errName != nil`, in either
// operand order and with nothing else attached.
func comparesNotNil(cond ast.Expr, errName string) bool {
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return false
	}

	return (isIdent(binary.X, errName) && isIdent(binary.Y, "nil")) ||
		(isIdent(binary.Y, errName) && isIdent(binary.X, "nil"))
}

// returnsOnlyIdent reports whether a block is EXACTLY one statement, that
// statement is a return, and that return carries the identifier errName among
// its results.
//
// # Why the statement COUNT is part of the shape
//
// The previous version looked for A return among the body's statements and let
// anything precede it. That is the difference between naming a part of the shape
// and constraining it, and three deformations walked through the gap while
// keeping the initialiser, the condition and a qualifying return intact:
//
//	if err := requirePathIDs(op, ...); err != nil { err = nil; return nil, err }
//	if err := requirePathIDs(op, ...); err != nil { var err error; return nil, err }
//	if err := requirePathIDs(op, ...); err != nil {
//		bad, badErr := f.tracer.DeleteRule(ctx, id)  // the REJECTED id, on the wire
//		_, _ = bad, badErr
//		return nil, err
//	}
//
// The first two hand the caller (nil, nil) for a REJECTED id — the silent-zero
// defect wearing a guard's clothes, the same one rule 4 closed for the return's
// own results, re-entered one statement earlier. The third is worse: the guard
// returns the honest validation error, and DELETE /v1/rules/{id} has already
// left with id=".." — the scope escalation two Epic-2 rounds were spent closing,
// now reachable from INSIDE the guard that exists to prevent it.
//
// A guard body is one statement in the live tree — all 202 of them, counted
// before this rule landed — so refusing everything else costs nothing. Anything
// a facade genuinely needs to do on a rejected id (log it, count it) belongs
// after the guard rejects, not between the rejection and the return.
func returnsOnlyIdent(body *ast.BlockStmt, errName string) bool {
	if body == nil || len(body.List) != 1 {
		return false
	}

	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}

	for _, result := range ret.Results {
		if isIdent(result, errName) {
			return true
		}
	}

	return false
}

// isIdent reports whether an expression is exactly the identifier name.
func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == name
}

// helperGuardsWhatItForwards reports whether a transition helper hands every id
// it forwards to the generated method to requirePathIDs.
//
// A transition helper receives the generated method as a function-typed
// parameter and calls it as a plain identifier, so the path positions cannot be
// resolved from the generated signatures the way a direct call's can. What can
// be checked is that nothing reaches that call unguarded: every plain identifier
// after the context must appear among the guarded values.
//
// Checking only that requirePathIDs is CALLED would leave the helpers open to
// the vacuous guard this scan exists to reject — and the helpers are the more
// expensive place to leave it open, because thirty-odd facade methods are
// credited on their promise rather than on a guard of their own.
func helperGuardsWhatItForwards(fn *ast.FuncDecl, pathOps map[string]bool) bool {
	callParam := functionTypedParameter(fn)
	if callParam == "" {
		return false
	}

	forwards := callsToIdent(fn.Body, callParam)
	if len(forwards) == 0 {
		// A helper that never invokes the method it was handed guards nothing,
		// however many pairs it names. Every method delegating to it is credited
		// on this promise, so an empty promise is a failure, not a pass.
		return false
	}

	// Same cutoff as a direct call site: a guard below the forwarding call is a
	// validation error about a request that already left.
	checked := guardedValues(fn, forwards[0].Pos())
	guarded := true

	for _, call := range forwards {
		// forwardedValues drops the context and the variadic editor tail and
		// compares everything else as source text. Its exclusions are sound here
		// because `call` is the GENERATED method the helper was handed: the tail is
		// oapi-codegen's request editors, never a path parameter.
		for _, value := range forwardedValues(fn, call, pathOps) {
			if !checked[value] {
				guarded = false
			}
		}
	}

	return guarded
}

// callsToIdent returns the calls in node whose target is the plain identifier
// name, in source order.
func callsToIdent(node ast.Node, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == name {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

// functionTypedParameter returns the name of fn's function-typed parameter — the
// generated method a transition helper is handed — or "" when it has none.
func functionTypedParameter(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}

	for _, field := range fn.Type.Params.List {
		if _, ok := field.Type.(*ast.FuncType); !ok || len(field.Names) == 0 {
			continue
		}

		return field.Names[0].Name
	}

	return ""
}

// pathArgumentNames reads the generated request builders and returns, per
// operation, the names of the builder parameters styled into the URL path.
//
// The identifier is taken rather than the wire name beside it: oapi-codegen
// writes StyleParamWithLocation("simple", false, "organization_id",
// runtime.ParamLocationPath, organizationId), and it is organizationId — the Go
// parameter — that lines up with the client method's signature.
func pathArgumentNames(t *testing.T, fset *token.FileSet) map[string]map[string]bool {
	t.Helper()

	args := map[string]map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Recv != nil {
					continue
				}

				op, ok := requestBuilderOperation(fn.Name.Name)
				if !ok {
					continue
				}

				if names := styledPathParameters(fn); len(names) > 0 {
					args[op] = names
				}
			}
		}
	}

	return args
}

// styledPathParameters returns the identifiers one request builder styles into
// the URL path.
func styledPathParameters(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 5 {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "StyleParamWithLocation" {
			return true
		}

		location, ok := call.Args[3].(*ast.SelectorExpr)
		if !ok || location.Sel.Name != "ParamLocationPath" {
			return true
		}

		if ident, ok := call.Args[4].(*ast.Ident); ok {
			names[ident.Name] = true
		}

		return true
	})

	return names
}

// clientMethodParameters returns the ordered parameter names of every generated
// *Client method, keyed by the exact method name.
//
// Exact rather than canonical: GetAssetByID and CreateAssetWithBody belong to
// different operations' argument lists, and a params struct or a content type
// shifts everything after it, so the positions have to come from the spelling
// the facade actually called.
func clientMethodParameters(t *testing.T, fset *token.FileSet) map[string][]string {
	t.Helper()

	params := map[string][]string{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Type.Params == nil {
					continue
				}

				if !isGeneratedClientReceiver(fn.Recv) {
					continue
				}

				params[fn.Name.Name] = parameterNames(fn.Type.Params)
			}
		}
	}

	return params
}

// isGeneratedClientReceiver reports whether the method hangs off *Client, the
// generated raw client. ClientWithResponses methods are the parser spellings and
// no facade may call them (see TestNoFacadeCallsAGeneratedParser).
func isGeneratedClientReceiver(recv *ast.FieldList) bool {
	if len(recv.List) == 0 {
		return false
	}

	typ := recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}

	ident, ok := typ.(*ast.Ident)

	return ok && ident.Name == "Client"
}

// parameterNames flattens a signature into one name per argument position, so a
// grouped "organizationId, ledgerId string" still yields two positions.
func parameterNames(params *ast.FieldList) []string {
	var names []string

	for _, field := range params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}

		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}

	return names
}

// operationsWithPathParameters reads the generated clients and returns the
// operations whose request builder formats at least one value into the URL
// path. oapi-codegen marks exactly those with runtime.ParamLocationPath.
func operationsWithPathParameters(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			collectPathOperations(file, ops)
		}
	}

	return ops
}

// collectPathOperations records every request builder in one generated file
// that formats a value into the URL path.
func collectPathOperations(file *ast.File, ops map[string]bool) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv != nil {
			continue
		}

		op, ok := requestBuilderOperation(fn.Name.Name)
		if !ok {
			continue
		}

		if formatsAPathParameter(fn) {
			ops[op] = true
		}
	}
}

// parseGoFiles parses the non-test Go files in dir, keyed by path.
//
// This is what go/parser's ParseDir used to do before it was deprecated, minus
// the package grouping these scans never needed: both callers want a flat list
// of files, and dropping the package level takes a loop out of each of them.
func parseGoFiles(t *testing.T, fset *token.FileSet, dir string) map[string]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	files := map[string]*ast.File{}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)

		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)

		files[path] = file
	}

	require.NotEmpty(t, files, "no non-test Go files found in %s; the scan is broken, not the code", dir)

	return files
}

// requestBuilderOperation maps a generated request-builder name back to its
// operation: NewGetBalanceByIDRequest and NewCreateAccountRequestWithBody both
// belong to the operation between the "New" and "Request".
func requestBuilderOperation(name string) (string, bool) {
	if !strings.HasPrefix(name, "New") {
		return "", false
	}

	trimmed := strings.TrimSuffix(name, "WithBody")
	if !strings.HasSuffix(trimmed, "Request") {
		return "", false
	}

	op := strings.TrimSuffix(strings.TrimPrefix(trimmed, "New"), "Request")

	return op, op != ""
}

// formatsAPathParameter reports whether a generated request builder styles a
// value into the URL path.
func formatsAPathParameter(fn *ast.FuncDecl) bool {
	found := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ParamLocationPath" {
			return true
		}

		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "runtime" {
			found = true
			return false
		}

		return true
	})

	return found
}

// pathOperationsNamedBy returns the path-parameter operations a node names,
// whether it CALLS the generated method or passes it as a function value to a
// transition helper. Both reach the same request builder, so both count.
//
// It takes any ast.Node — a function body, or a whole package-level declaration
// — so the same matching covers both scopes.
func pathOperationsNamedBy(node ast.Node, pathOps map[string]bool) []string {
	seen := map[string]bool{}

	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if op, ok := generatedOperation(sel.Sel.Name); ok && pathOps[op] {
			seen[op] = true
		}

		return true
	})

	ops := make([]string, 0, len(seen))
	for op := range seen {
		ops = append(ops, op)
	}

	sort.Strings(ops)

	return ops
}

// generatedOperation strips the wrappers oapi-codegen puts around an operation
// name in the FIVE spellings it emits: Op and OpWithBody on the raw *Client,
// OpWithResponse and OpWithBodyWithResponse on *ClientWithResponses, and the
// exported free function ParseOpResp that the last two delegate to.
//
// The free function is not a curiosity, and leaving it out was a real hole:
// genledger exports 197 of them and gentracer 28, and
// GetSegmentByIDWithResponse IS the raw call followed by
// ParseGetSegmentByIDResp. A facade doing those two steps by hand reproduces
// the banned parser byte for byte under a name no WithResponse suffix match
// would ever see.
//
// The Parse/Resp pair is unambiguous here because no generated operation is
// itself named Parse* — checked against both clients' request builders, which
// are the only place an operation name is minted.
func generatedOperation(name string) (string, bool) {
	if parsed, ok := strings.CutPrefix(name, "Parse"); ok {
		op, ok := strings.CutSuffix(parsed, "Resp")

		return op, ok && op != ""
	}

	op := strings.TrimSuffix(name, "WithResponse")
	op = strings.TrimSuffix(op, "WithBody")

	return op, op != ""
}

// isParserSpelling reports whether a name reaches oapi-codegen's response
// parser — the *ClientWithResponses method, or the free function it delegates
// to. Both unmarshal the body before any facade logic runs, which is the failure
// this package bans; the two spellings are one behaviour and are judged as one.
func isParserSpelling(name string) bool {
	return strings.HasSuffix(name, "WithResponse") ||
		(strings.HasPrefix(name, "Parse") && strings.HasSuffix(name, "Resp"))
}

// callsAny reports whether the function calls a package-local function whose
// name satisfies match.
func callsAny(fn *ast.FuncDecl, match func(string) bool) bool {
	found := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fun := call.Fun

		// A generic helper reads as reservationTransition[T].
		if idx, ok := fun.(*ast.IndexExpr); ok {
			fun = idx.X
		}

		ident, ok := fun.(*ast.Ident)
		if !ok {
			return true
		}

		if match(ident.Name) {
			found = true
			return false
		}

		return true
	})

	return found
}

// funcLabel names a function the way a reader would: Type.Method, or the bare
// name for a package-level function.
func funcLabel(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}

	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}

	if ident, ok := recv.(*ast.Ident); ok {
		return ident.Name + "." + fn.Name.Name
	}

	return fn.Name.Name
}
