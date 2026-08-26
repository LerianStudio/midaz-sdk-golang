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

// pathIDGuard is the shared pre-request guard every facade runs on the values it
// formats into a URL path. It is the only spelling this scan reads; a facade
// validating its ids some other way is not credited, by design.
const pathIDGuard = "requirePathIDs"

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
//     REPORTED — see unaccountedMentions, which carries that deleted branch's
//     four escapes and why closing them in place bought nothing;
//   - the operation NAMED but never directly called, standing beside an honest
//     guard and an honest direct read. The report above was written but was
//     unreachable: any direct call at all sent the function down the direct
//     branch, which examined the direct calls and credited, dropping every
//     operation it had not been handed. Closed by reporting what no examined
//     call accounts for, on EVERY function rather than on the branch that used
//     to fall through to it — see unaccountedMentions and offences;
//   - the operation mentioned TWICE, once honestly and once not. The report just
//     described was keyed by operation NAME — a set of the names some call
//     reached, compared against the names the function mentions — so ONE honest
//     mention laundered every other mention of the SAME name in the same
//     function. An operation called and guarded, plus a second handoff of that
//     same operation to an unknown helper alongside a different, UNGUARDED id,
//     was invisible; so was the transition-helper spelling of it, and so was a
//     different operation buried one level inside a transition call's arguments,
//     which the union credited because it walked the whole CallExpr. Closed by
//     making "accounted" a property of the mention NODE rather than of the
//     symbol — see unaccountedMentions.
//
// # Known ceiling, and it is deliberate
//
// Five things remain out of reach, and faking them with heuristics would be
// worse than saying so:
//
//   - a guarded variable REASSIGNED between the guard and the call —
//     requirePathIDs(op, "id", id), then id = "..", then f.ledger.GetX(ctx, id);
//   - an inner scope SHADOWING a name this scan matches by SPELLING, and there
//     are three families of such name, not one. The guarded VALUES: the
//     identifier the call forwards is a different variable wearing the spelling
//     the guard received. The guard FUNCTION itself: a local
//     `requirePathIDs := func(...) error { return nil }` makes every guard below
//     it vacuous while every part of the four-part shape stays perfect, so the
//     scan credits it. And a TRANSITION HELPER's name: a local
//     `ruleTransition := func(...) { ... }` is matched against transitionHelpers
//     by spelling too, so the delegation inherits the real helper's credit —
//     helperGuardsWhatItForwards checks the package-level declaration while the
//     call site reaches the closure, and the promise the credit rests on was
//     made by code that never runs. All three are one defect — an identifier
//     matched by SPELLING against a definition the scan never resolves — and one
//     tool (identifier resolution) closes all three;
//   - the guard reached through a VALUE rather than by name: `g := requirePathIDs`
//     and then `g(op, "id", id)`. deformedGuardCalls matches call.Fun against the
//     literal identifier requirePathIDs, so a hoisted guard is not a guard call
//     to it at all — neither accepted nor refused, simply unseen. That re-enters
//     the silent zero the round-5 refusal closed, by the order it closed: a
//     DEFORMED hoisted guard (`if err := g(...); err != nil { err = nil;
//     return nil, err }`) written above a well-shaped direct one credits via the
//     second while the runtime takes the first. Nothing in the tree hoists a
//     guard function, and doing so carries no innocent meaning in a facade;
//   - a REQUEST EDITOR rewriting the outbound path. Every generated method takes
//     a `...RequestEditorFn` tail, and an editor is
//     `func(context.Context, *http.Request) error` — it holds the built request
//     and can assign to req.URL.Path, on any branch, AFTER the guard has run and
//     after the request builder styled its parameters. A closure capturing a
//     caller's value puts that value in the path without it ever sitting at a
//     path ARGUMENT POSITION, which is the only place pathArgumentsOf looks. This
//     is not hypothetical scaffolding: `setQueryParam`
//     (organizations_facade.go:243-252) is a hand-written editor in this very
//     package. It is harmless — it touches only req.URL.RawQuery, and its single
//     call site passes literals — but it is the shape, one field away;
//   - REFLECTION — a client method resolved by name at runtime, which is
//     invisible to AST matching. This is the ceiling all three structural scans
//     in this package share; nothing here does it, and none of the three pretends
//     to cover it.
//
// The first three are invisible to a scan that compares source text, and closing
// any of them honestly needs IDENTIFIER RESOLUTION — a type-checked pass that
// resolves every name to its definition, and for the first, SSA on top of it to
// prove no assignment reaches the call. A stricter text match cannot do it: the
// hostile spelling and the honest one are the same characters, which is the
// whole shape of all three. The fourth needs the same machinery pointed
// elsewhere: resolving every value in an editor tail to the function it names
// and proving no assignment in that body reaches req.URL. All four are a
// different tool, not a stricter match.
//
// # Accepted strictness, so nobody "fixes" it
//
// Nine false positives are deliberate, and each is the price of a match that
// cannot be talked into accepting a hostile input. Each one names the one-line
// change a contributor makes at the site; none of them is a reason to widen the
// matcher, because every widening that admits one of these admits a shape from
// the deformation tables above with it.
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
//   - A NAKED RETURN under named results is reported:
//     `func f(...) (rule *models.Rule, err error) { if err = requirePathIDs(...);
//     err != nil { return } ... }` is ordinary Go propagation, and it refuses
//     because the return carries NO results, so rule 4 has nothing to match. That
//     is the point rather than an oversight: rule 4 exists to pin what flows out
//     of the guard, and a naked return makes what flows out unreadable from the
//     source — it hands back whatever the named results hold at that instant,
//     which is the same thing `err = nil; return nil, err` did. Reading it needs
//     dataflow, not a wider match. No live site uses it; all 202 name their
//     results. Fix at the site: `return nil, err`.
//   - A BLOCK-WRAPPED return is reported: `if err := requirePathIDs(...);
//     err != nil { { return nil, err } }` refuses, because the body's single
//     statement is a block, not a return. Accepting it means descending through
//     nested blocks to find A return, which is the "a return somewhere in the
//     body" rule round 4 deleted — it is what let a rejected id reach the wire
//     from INSIDE the guard, one statement before the return. Fix at the site:
//     drop the inner braces.
//   - A REDUNDANT NESTED guard is reported: a perfectly-shaped guard inside an
//     if, an else-if, a loop or a closure, standing beside a depth-1 guard that
//     already covers the same value. The reasoning, and the alternative rule that
//     was weighed against it, are on deformedGuardCalls. Fix at the site: hoist
//     it, or delete it as the duplicate it is.
//   - A HOISTED OPERATION is reported even when it is honestly called and
//     honestly guarded: `get := f.ledger.GetAccountByID` followed by
//     `get(ctx, orgID, ledgerID, id)` is refused, because the mention on the
//     right-hand side of the binding is neither the target of a direct call nor
//     an argument of a transition call — the call target is the local. Accepting
//     it means crediting the mention because some LOCAL derived from it is later
//     called, which is the name-keyed laundering of unaccountedMentions one
//     level of indirection along: bind once, call the local honestly, and hand
//     the SAME local to an unknown helper with an unguarded id, and the second
//     handoff is an identifier this scan cannot attribute at all. directPathCalls
//     still follows the binding, so the arguments of the hoisted call are still
//     compared — that can only ADD findings, never remove one, and it can no
//     longer change a verdict, since the binding itself is already reported. Fix
//     at the site: call the operation directly.
//   - A PARENTHESISED transition-helper NAME is reported:
//     `(ruleTransition)(ctx, op, f.tracer.ActivateRule, id)` refuses, because
//     calleeName reads through a generic helper's type arguments and nothing
//     else, so the call is not recognised as a transition call at all and the
//     operation it hands over becomes an unaccounted mention. That is the same
//     "nothing else is read through" rule accountedMentionNodes states for the
//     ARGUMENT side, and it stays strict here for a reason the argument side
//     does not share: unparenthesising an ARGUMENT credits the one node that
//     would have been credited anyway, while unparenthesising a CALLEE turns a
//     call this scan does not know into a transition call, which credits every
//     element of its argument list at once. Widening credit and widening a
//     report are not the same risk. Fix at the site: drop the parentheses.
//   - A MULTI-VALUE CALL EXPANSION is reported:
//     `f.ledger.GetSegmentByID(probeScope(ctx, orgID, ledgerID, id))` — Go's
//     spelling for forwarding one call's results as another call's whole
//     argument list — refuses, because the call carries ONE argument expression
//     while the path positions sit at 1, 2 and 3, so no expression can be placed
//     at a path position and there is nothing to compare against the guard.
//     Accepting it means reading through the inner call to the values it
//     returns, which is dataflow rather than a match. Until fix round 7 this
//     shape was not a false positive at all — it was CREDITED, silently, and
//     pathArgumentsOf carries the walkthrough. Nothing in the tree spells a call
//     this way. Fix at the site: pass the arguments directly.
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

	// The other two derived inputs get the same fail-closed check, and they get it
	// because they did NOT have one. On the oapi-codegen v2.7 upgrade the styled
	// path reading broke and pathArgs came back empty, which is a BROKEN SCAN; with
	// no assertion here it surfaced instead as 205 individual facade offences
	// blaming the *Client signatures. A reader sent to the wrong half of the scan
	// 205 times over is the cost of deriving a universe and not asserting you found
	// one.
	pathArgs := pathArgumentNames(t, fset)
	require.NotEmpty(t, pathArgs,
		"found no request builder styling a value into the URL path; the scan is broken, not the code")

	methodParams := clientMethodParameters(t, fset)
	require.NotEmpty(t, methodParams,
		"found no generated *Client method signatures; the scan is broken, not the code")

	scan := pathGuardScan{
		fset:         fset,
		pathOps:      pathOps,
		pathArgs:     pathArgs,
		methodParams: methodParams,
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
//
// # No branch decides on its own
//
// An earlier version chose ONE check per function: any direct call at all sent
// it down the direct branch, which examined the direct calls and credited. An
// operation the function NAMED but did not directly call was dropped on that
// path, and the unknown-delegation report below was unreachable from it. The
// deformation that proved it kept an honest guard and an honest direct read, and
// handed a generated operation plus an UNGUARDED id to a package-local helper
// beside them: the function was credited, every structural scan stayed green,
// and the wire carried the unguarded id.
//
// So the checks are no longer alternatives. Every function runs every check and
// the offences are reported together — see offences, which is also why the
// deformed-guard check reaches a transition helper, a function that names no
// operation at all.
func (s pathGuardScan) classify(path string, decl ast.Decl, helperGuards map[string]bool) (string, int) {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return s.packageScopeBinding(path, decl), 0
	}

	if transitionHelpers[fn.Name.Name] {
		helperGuards[fn.Name.Name] = helperGuardsWhatItForwards(fn, s.pathOps)
	}

	site := " (declared at " + filepath.Base(path) + ":" +
		strconv.Itoa(s.fset.Position(fn.Pos()).Line) + ")"

	ops := pathOperationsNamedBy(fn.Body, s.pathOps)
	calls := directPathCalls(fn, s.pathOps)

	if problems := s.offences(fn, calls, site); len(problems) > 0 {
		return strings.Join(problems, "; "), 0
	}

	if len(ops) == 0 {
		return "", 0
	}

	return "", 1
}

// offences runs every check on one function and returns all of them, rather than
// picking a branch by the shape of what it found.
//
// The order is reporting order, not precedence: nothing here returns early, so
// a MENTION of an operation that no examined call accounts for is reported even
// when the calls the function DOES make are perfectly guarded — which is the
// point, since the offending mention and the honest one are routinely the same
// operation.
func (s pathGuardScan) offences(fn *ast.FuncDecl, calls []pathCall, site string) []string {
	var problems []string

	// First, because it is the only check that runs on a function naming no
	// operation at all — which is exactly what a transition helper is.
	if deformed := deformedGuardCalls(fn); len(deformed) > 0 {
		problems = append(problems, s.deformedGuardOffence(fn, deformed, site))
	}

	if mentions := unaccountedMentions(fn, calls, s.pathOps); len(mentions) > 0 {
		problems = append(problems, s.unaccountedMentionOffence(fn, mentions, site))
	}

	if missing := s.unguardedPathArguments(fn, calls); len(missing) > 0 {
		problems = append(problems, funcLabel(fn)+" forwards "+strings.Join(missing, ", ")+
			" into a URL path without handing it to requirePathIDs"+site)
	}

	return problems
}

// unaccountedMentions returns the mentions of a generated path-parameter
// operation inside fn that no examined call accounts for — one entry per
// OCCURRENCE, not one per operation.
//
// Two positions account for a mention, and both are properties of the NODE:
//
//   - the node is the Fun of a DIRECT call, whose arguments
//     unguardedPathArguments then compares against the guard; or
//   - the node is ITSELF a direct element of the argument list of a call to a
//     KNOWN transition helper, whose interior helperGuardsWhatItForwards checks
//     separately.
//
// Every other mention is reported, same-named or not, nested or top-level: a
// function value handed to a package-local helper the scan does not know, an
// operation bound to a local, an operation buried one level inside an argument
// of a delegating call, an operation named and never invoked. That is the same
// unknown=flag rule unguardedPathArguments applies to an unrecognised generated
// spelling.
//
// # Accounted is a property of the OCCURRENCE, never of the SYMBOL
//
// The previous version keyed accounting by operation NAME. It built a set of the
// names reached by a direct call, unioned the names appearing ANYWHERE inside a
// call to a known transition helper, and reported the names of the function's
// mentions missing from that set. A set keyed by name collapses every distinct
// mention of one operation into a single entry, so ONE honest mention laundered
// every other mention of that name in the same function. Three shapes walked
// through — all compiling, all credited, all putting an unguarded id on the
// wire:
//
//   - DeleteRule called directly and guarded honestly, plus a SECOND handoff of
//     f.tracer.DeleteRule to a package-local helper alongside an UNGUARDED
//     cascade id. The direct call had already put the NAME in the set, so the
//     handoff was invisible and DELETE /v1/rules/{id} left with id="..", the
//     scope escalation two Epic-2 rounds were spent closing. Its read-only twin
//     — an honest GetRule read beside a fallback handoff of the same GetRule —
//     behaves identically, so nothing about it depends on the delegated call
//     being destructive;
//   - the same laundering with the honest mention credited by a TRANSITION
//     helper instead of by a direct call: ruleTransition(ctx, op,
//     f.tracer.ActivateRule, id) standing beside sneak(f.tracer.ActivateRule,
//     otherID). The transition call accounted for the name, and POST
//     /v1/rules/{id}/activate carried otherID unguarded. This function has NO
//     direct call at all, so it is also the shape that takes the one early exit
//     in unguardedPathArguments — see the comment at that exit;
//   - a DIFFERENT operation nested inside the transition call's own arguments.
//     The union walked the ENTIRE CallExpr with pathOperationsNamedBy, so an
//     operation buried in a nested call at an argument position was credited to
//     a helper that never receives it.
//
// There is no name match strict enough to close this, because the offending
// mention and the honest one carry the same name by construction. A mention
// either sits at one of the two positions above or it does not, and that is
// decided per node.
//
// # Why an unreadable delegation is REPORTED rather than judged in place
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
// unrecognised generated spelling. All four escapes close by construction.
//
// The cost is the friction transitionHelpers exists to create: a facade
// delegating to a NEW local helper fails until someone adds that helper to
// transitionHelpers, where helperGuardsWhatItForwards then checks its interior.
// The list is literal on purpose — "adding a fourth helper here should be a
// decision someone makes on purpose" — and this is what makes that decision
// unavoidable rather than optional.
func unaccountedMentions(fn *ast.FuncDecl, calls []pathCall, pathOps map[string]bool) []*ast.SelectorExpr {
	accounted := accountedMentionNodes(fn, calls)

	var unaccounted []*ast.SelectorExpr

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || accounted[sel] {
			return true
		}

		if op, ok := generatedOperation(sel.Sel.Name); ok && pathOps[op] {
			unaccounted = append(unaccounted, sel)
		}

		return true
	})

	return unaccounted
}

// accountedMentionNodes returns the exact NODES a call accounts for: the target
// of every direct call, and every direct element of a known transition helper's
// argument list.
//
// Parentheses are read through on the delegating side for the reason isIdent
// gives — they carry no meaning, and refusing `ruleTransition(ctx, op,
// (f.tracer.ActivateRule), id)` would be a pure false positive. Nothing else is
// read through: an operation wrapped in a conversion or in another call is a
// value the receiving helper computes, not one this scan can attribute to it.
func accountedMentionNodes(fn *ast.FuncDecl, calls []pathCall) map[ast.Expr]bool {
	accounted := map[ast.Expr]bool{}

	for _, pc := range calls {
		accounted[pc.call.Fun] = true
	}

	for _, call := range transitionHelperCalls(fn) {
		for _, arg := range call.Args {
			accounted[ast.Unparen(arg)] = true
		}
	}

	return accounted
}

// transitionHelperCalls returns the calls fn makes to a helper in
// transitionHelpers, so the operations handed over in those calls can be
// credited to the helper's own guard.
//
// The operation must be named IN the delegating call. An earlier version asked
// only whether the function called a transition helper ANYWHERE, which credited
// every operation the function named the moment one of them was delegated.
func transitionHelperCalls(fn *ast.FuncDecl) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if name, ok := calleeName(call.Fun); ok && transitionHelpers[name] {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

// calleeName returns the plain-identifier name a call targets, reading through
// the type arguments a generic helper carries (reservationTransition[T], and the
// multi-parameter spelling go/ast models as an IndexListExpr).
func calleeName(fun ast.Expr) (string, bool) {
	switch node := fun.(type) {
	case *ast.IndexExpr:
		fun = node.X
	case *ast.IndexListExpr:
		fun = node.X
	}

	ident, ok := fun.(*ast.Ident)
	if !ok {
		return "", false
	}

	return ident.Name, true
}

// unaccountedMentionOffence names each unaccounted mention with the LINE it sits
// on, because one function can mention the same operation several times and the
// reader needs the occurrence, not the name. Naming the operation alone is what
// the name-keyed model did, and it is exactly the distinction that model lost.
func (s pathGuardScan) unaccountedMentionOffence(
	fn *ast.FuncDecl,
	mentions []*ast.SelectorExpr,
	site string,
) string {
	named := make([]string, 0, len(mentions))
	for _, sel := range mentions {
		named = append(named, sel.Sel.Name+" on line "+strconv.Itoa(s.fset.Position(sel.Pos()).Line))
	}

	return funcLabel(fn) + " mentions " + strings.Join(named, ", ") +
		" without calling it there or handing it to a helper in transitionHelpers, so no guard can" +
		" be verified for that mention — call the operation directly, or add the helper that" +
		" receives it to that list, where its interior is checked" + site
}

// forwardedValues returns the arguments of one call that could carry a caller's
// value into a URL path, as SOURCE TEXT — the same spelling guardedValues
// records, so the two are compared as written.
//
// Its one caller is helperGuardsWhatItForwards, so the call it reads is always a
// call to a KNOWN transition helper's function-typed parameter: the generated
// method itself. That is what makes the exclusions below sound — see
// unaccountedMentions for the deleted branch that applied these same exclusions
// to an arbitrary local helper, where two of them stopped being true, and for
// why that branch is gone rather than repaired.
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
//     costs nothing to drop. By NAME is a weaker rule than by TYPE — a value
//     spelled like the context parameter is dropped without ever being compared
//     — and that residual is the fourth of the four escapes recorded on
//     unaccountedMentions. It survives here because the three helpers this now
//     runs on are a literal list a human curates.
//
//   - a LITERAL, and a local `const operation = "Segments.Get"`. Both are fixed
//     at compile time; the const is the error label every facade declares, and it
//     cannot carry ".." in from a caller. Requiring it to be guarded would be a
//     false positive on the correctly-written form of the very shape this check
//     exists to catch.
//
//   - the GENERATED OPERATION, when a helper forwards one (`f.tracer.ActivateRule`).
//     It is a function value, matched by resolving the selector against pathOps,
//     which is why `ids.ID` — a selector too — is not excluded with it.
//
//   - the VARIADIC SPREAD at the tail (`idempotencyEditorsTracer(ctx, false)...`,
//     which all three live helpers forward). oapi-codegen never puts a path
//     parameter in a GENERATED method's variadic tail; that tail is the request
//     editors, always — and a generated method is the only thing the surviving
//     caller reads, which is exactly what makes this exclusion sound here and
//     made it unsound in the deleted branch.
//
//     That argument is about the SIGNATURE, and it is worth being explicit that
//     it is not the stronger claim it can read as. It says nothing an editor
//     cannot do: an editor holds the built *http.Request and can write req.URL,
//     so the tail CAN reach the path even though nothing in it is a path
//     parameter. Excluding it here is right — comparing an editor's source text
//     against guarded values would report every live call site and prove nothing
//     — and what the exclusion costs is recorded honestly as the third entry in
//     the known ceiling above, where closing it needs the editor's BODY resolved
//     and read, not a stricter match here.
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
// call.Fun counts zero calls — which, before the checks stopped being
// alternatives, dropped the function into the weakest branch. That is the hoist
// the sibling delete-seam scan was defeated by in Epic 3 and closed against;
// this is the same closure, ported. Binding a generated operation to a local
// carries no innocent meaning in a facade, so the binding is simply followed.
//
// Since accounting moved to mention NODES, a hoist is REPORTED on its own —
// the right-hand side of the binding is neither a call target nor a transition
// argument, so unaccountedMentions names it (accepted-strictness list, entry
// seven). This function can therefore no longer change a verdict; it survives
// because following the binding still puts the hoisted call's arguments in front
// of the identity check, which can only add findings to a function that is
// already reported.
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
//
// A call whose spelling IS resolvable can still be unreadable, for a second
// reason: its argument list may be too short to place the path positions the
// signature names. That is reported too, and why is on pathArgumentsOf.
func (s pathGuardScan) unguardedPathArguments(fn *ast.FuncDecl, calls []pathCall) []string {
	// No direct call, no argument expressions to compare — this is the one exit
	// in the file that skips a check, and what it skips is provably empty for the
	// inputs that reach it. Re-derived when accounting moved from operation NAMES
	// to mention NODES, because a function with zero direct calls still has
	// mention nodes and the exit must not swallow them: it does not.
	// unaccountedMentions walks fn.Body itself, independently of `calls`, and
	// offences calls it on EVERY function; the delegating shape that takes this
	// exit — a transition delegation beside a handoff to an unknown helper, no
	// direct call anywhere — is reported there, per mention. What this return
	// declines is the loop below, which reads call.Args, of which there are none.
	if len(calls) == 0 {
		return nil
	}

	checked := guardedValues(fn, earliestCall(calls))

	var missing []string

	for _, pc := range calls {
		op, _ := generatedOperation(pc.method)

		names, wanted := s.methodParams[pc.method], s.pathArgs[op]
		if len(names) == 0 || len(wanted) == 0 {
			missing = append(missing, unresolvableCallOffence(pc, op, len(names) == 0))

			continue
		}

		args, placed := pathArgumentsOf(pc.call, names, wanted)
		if !placed {
			missing = append(missing, s.unplaceableCallOffence(pc, op))
		}

		for _, arg := range args {
			if !checked[arg] {
				missing = append(missing, arg+" (into "+op+")")
			}
		}
	}

	sort.Strings(missing)

	return missing
}

// unresolvableCallOffence names a call whose path positions cannot be resolved,
// and says WHICH of the two derivations came back empty for it.
//
// Saying which is the whole point of splitting this out. The two causes used to
// share one message that named only the signature half, and when the styled-path
// reading went blind on the oapi-codegen v2.7 upgrade that message blamed the
// *Client signatures — which were byte-identical to the version before — on all
// 205 operations at once. The emptiness checks in the test body now catch a
// wholesale break before it reaches here; this keeps a SINGLE unresolvable
// operation honest about its cause.
func unresolvableCallOffence(pc pathCall, op string, noSignature bool) string {
	if noSignature {
		return "every path argument of " + op + " — reached through " + pc.method +
			", a spelling with no raw *Client signature, so this scan cannot resolve which " +
			"arguments become path segments —"
	}

	return "every path argument of " + op + " — whose request builder styles no value into the URL " +
		"path, so this scan cannot resolve which of " + pc.method + "'s arguments become path segments —"
}

// unplaceableCallOffence names a call whose argument list cannot place every
// path position, with the LINE it sits on — one function can call the same
// operation several times and only one of those calls may be the unreadable one,
// which is the occurrence-not-symbol distinction the mention report already
// carries.
func (s pathGuardScan) unplaceableCallOffence(pc pathCall, op string) string {
	return "every path argument of " + op + " — called on line " +
		strconv.Itoa(s.fset.Position(pc.call.Pos()).Line) + " with an argument list too short to place " +
		"the path positions " + pc.method + " declares, which is what a multi-value call expansion " +
		"looks like, so this scan cannot read which expressions become path segments —"
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
// generated client styles into the URL path, and whether every WANTED position
// could be placed in that call's argument list.
//
// names are the ordered parameter names of the exact spelling at the call site
// (GetAssetByID, CreateAssetWithBody, ...), because the argument LIST differs
// between an operation's spellings; wanted are the parameter names its request
// builder styles into the path.
//
// # An UNPLACEABLE position is unreadable, and unreadable is a FLAG
//
// The previous version skipped a wanted position sitting past the end of
// call.Args, on an assumption it never stated: that a short argument list only
// ever means the caller omitted the variadic request-editor tail. Go's
// multi-value expansion breaks that assumption outright.
// `f.ledger.GetSegmentByID(probeScope(ctx, orgID, ledgerID, id))` compiles,
// forwards four values, and has len(call.Args) == 1 — so EVERY wanted position
// was past the end, every one was skipped, `missing` came back empty and the
// function was CREDITED. Wire-proven with orgID = "..", which pops a path
// segment and turns a scoped read into a cross-organization one. An honest
// guard written above such a call does not save it: the scan credits either
// way, so nothing stops a later refactor from deleting a comparison that was
// never made.
//
// The bound is on the highest WANTED index and never on len(names). A call that
// legitimately omits the editor tail leaves len(call.Args) < len(names) and is
// perfectly readable, and flagging every write that carries editors would bury
// the scan in false positives. What is refused is narrower and exact: a call
// whose argument list cannot place a PATH position is a call whose path
// segments cannot be identified, so it is reported as unreadable rather than
// waved through. Same unknown=flag rule unguardedPathArguments applies to an
// unrecognised generated spelling, one level down.
func pathArgumentsOf(call *ast.CallExpr, names []string, wanted map[string]bool) ([]string, bool) {
	var args []string

	placed := true

	for i, name := range names {
		if !wanted[name] {
			continue
		}

		if i >= len(call.Args) {
			placed = false

			continue
		}

		args = append(args, types.ExprString(call.Args[i]))
	}

	return args, placed
}

// deformedGuardCalls returns the requirePathIDs calls in fn that are NOT the
// initialiser of an accepted guard at depth 1 of the function body.
//
// # A deformed guard is a FLAG, not an absence
//
// guardedValues walks the function's own statements and collects the values
// behind each guard it accepts. Every earlier version simply SKIPPED a statement
// whose guard did not match — treating a deformed call as if it were not there —
// and that made the verdict depend on ORDER:
//
//	if err := requirePathIDs(op, "id", id); err != nil { err = nil; return nil, err }
//	if err := requirePathIDs(op, "id", id); err != nil { return nil, err }
//	resp, err := f.tracer.GetRule(ctx, id)
//
// The first guard is refused (its body is two statements, so it hands the caller
// a nil error for a rejected id) and skipped; the second is accepted and credits
// id; the scan passes. At RUNTIME the FIRST one executes, and `id=".."` reaches
// the wire with err=nil — the silent zero and the scope escalation, reached past
// a scan that had already refused the very statement doing it. Either order
// works, because skipping is order-blind in both directions.
//
// So refusal now means refusal: a guard call the accepted shape does not cover
// makes the function an offender on its own, whatever else in it guards. This is
// the unknown=flag rule the round-4 deletion applied to delegations, applied to
// guard SPELLINGS — "refuse everything else by construction" is not satisfied by
// looking away.
//
// # What "accepted" means here, and the nesting decision
//
// Accepted is guardCallActedOn's four-part shape AND depth 1 of the function
// body — the two requirements guardedValues already imposed together. So a
// guard nested inside an if, an else-if, a loop, a switch case or a closure is
// now REPORTED rather than silently uncredited, even when its own four parts are
// perfect and another depth-1 guard already covers the same value.
//
// That is a deliberate choice between two defensible rules, and it is the
// stricter one. The alternative — judge the four parts wherever the guard sits,
// and let depth decide only whether it CREDITS — keeps a redundant nested guard
// clean, at the cost of a second, weaker acceptance test living beside the first.
// Two tests for one shape is how the last four rounds went wrong. The strict rule
// costs nothing live: all 202 guards in the tree are statements of their
// function's own body, counted mechanically before this landed, and a facade that
// genuinely wants a conditional check can put it after the unconditional one.
func deformedGuardCalls(fn *ast.FuncDecl) []*ast.CallExpr {
	accepted := map[*ast.CallExpr]bool{}

	for _, stmt := range fn.Body.List {
		if call, ok := guardCallActedOn(stmt, pathIDGuard); ok {
			accepted[call] = true
		}
	}

	var deformed []*ast.CallExpr

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isIdent(call.Fun, pathIDGuard) || accepted[call] {
			return true
		}

		deformed = append(deformed, call)

		return true
	})

	return deformed
}

// deformedGuardOffence names the lines the refused guard calls sit on, because
// the function may hold several and the reader needs the one to look at.
func (s pathGuardScan) deformedGuardOffence(fn *ast.FuncDecl, calls []*ast.CallExpr, site string) string {
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		lines = append(lines, strconv.Itoa(s.fset.Position(call.Pos()).Line))
	}

	return funcLabel(fn) + " calls " + pathIDGuard + " on line " + strings.Join(lines, ", ") +
		" in a shape this scan does not accept — the only credited spelling is" +
		" `if err := " + pathIDGuard + "(...); err != nil { return ..., err }`," +
		" written as a statement of the function's own body" + site
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
//
// A statement this skips is not thereby forgiven. Skipping used to be the whole
// verdict on a refused guard, which let a deformed one hide behind a well-shaped
// sibling written after it; deformedGuardCalls now reports every guard call this
// loop would decline, so this function only has to answer what was CHECKED, not
// what was attempted.
func guardedValues(fn *ast.FuncDecl, before token.Pos) map[string]bool {
	values := map[string]bool{}

	for _, stmt := range fn.Body.List {
		if stmt.Pos() >= before {
			break
		}

		call, ok := guardCallActedOn(stmt, pathIDGuard)
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
// operand order, with nothing else attached and through any redundant
// parentheses.
func comparesNotNil(cond ast.Expr, errName string) bool {
	binary, ok := ast.Unparen(cond).(*ast.BinaryExpr)
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

// isIdent reports whether an expression is exactly the identifier name, reading
// through any redundant parentheses around it.
//
// Parentheses carry no meaning here — `(err) != (nil)` and `return nil, (err)`
// are the same guard as the live spelling — so refusing them is a pure false
// positive, and one that reads as a real finding to whoever hits it. Unparen at
// every position the matcher inspects admits no value it would otherwise refuse:
// `ast.Unparen` strips grouping and nothing else.
func isIdent(expr ast.Expr, name string) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)

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
// writes StyleParamWithOptions("simple", false, "organization_id",
// organizationId, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, ...}),
// and it is organizationId — the Go parameter — that lines up with the client
// method's signature, while "organization_id" is the wire spelling and lines up
// with nothing here. See styledPathParameters for the argument positions and how
// they moved in oapi-codegen v2.7.
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
//
// # Old shape → new shape (oapi-codegen v2.4 → v2.7)
//
// v2.4 passed the parameter location as a POSITIONAL argument, with the value
// last:
//
//	runtime.StyleParamWithLocation("simple", false, "id", runtime.ParamLocationPath, id)
//	                                                     └ Args[3] location     └ Args[4] value
//
// v2.7 retired that helper for one taking an options struct, which moves the
// value one position LEFT and buries the location inside a composite literal:
//
//	runtime.StyleParamWithOptions("simple", false, "id", id,
//	    runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
//	                                                    └ Args[3] value  └ Args[4] options
//
// Reading the old positions against the new call finds the OPTIONS STRUCT where
// the location used to be, so the location check fails on every builder and this
// map comes back empty for the whole client. It did, on the upgrade: the scan
// reported all 205 path operations as "a spelling with no raw *Client
// signature" — the wrong fact, since those signatures were byte-identical and it
// was this reading that had gone blind. unresolvableCallOffence now separates
// the two causes so that misreport cannot recur.
//
// The location is read from the struct FIELD BY NAME rather than from a position
// inside the literal, so the next field oapi-codegen adds to StyleParamOptions
// cannot shift it — Type and Format arrived with the struct itself.
func styledPathParameters(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 5 {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "StyleParamWithOptions" {
			return true
		}

		if !stylesIntoThePath(call.Args[4]) {
			return true
		}

		if ident, ok := call.Args[3].(*ast.Ident); ok {
			names[ident.Name] = true
		}

		return true
	})

	return names
}

// stylesIntoThePath reports whether a runtime.StyleParamOptions literal sets
// ParamLocation to runtime.ParamLocationPath.
//
// Only a KEYED field named ParamLocation counts. A positional literal is not
// read, because guessing which position the location occupies is what the
// v2.4-shaped reading above was doing when it went blind.
func stylesIntoThePath(arg ast.Expr) bool {
	lit, ok := arg.(*ast.CompositeLit)
	if !ok {
		return false
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "ParamLocation" {
			continue
		}

		if location, ok := kv.Value.(*ast.SelectorExpr); ok {
			return location.Sel.Name == "ParamLocationPath"
		}
	}

	return false
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
