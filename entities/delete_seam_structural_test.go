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
// the response to deleteResource. Reaching for a generated PARSER spelling is
// refused outright, because that spelling IS the broken parser. Both spellings
// count — the *WithResponse method and the ParseDelete*Resp free function it
// delegates to — through the shared isParserSpelling.
//
// Derived from the source at test time rather than from a checked-in list, so a
// delete added tomorrow is covered when it lands rather than when someone
// remembers it.
//
// # Where the delete universe comes from
//
// deleteOperations DERIVES it by parsing ../internal/genledger and
// ../internal/gentracer and keeping every request builder whose body issues
// DELETE. Nothing here is hand-maintained, so there is no list to forget to
// update and the seamed floor below is a backstop against the SCAN breaking,
// not against a missed entry.
//
// The residual is the reading, not the coverage: the method is matched in the
// builder's SOURCE TEXT as the literal http.NewRequest("DELETE", because that is
// how oapi-codegen writes it. A generated builder that computed its method — from
// a variable, a constant, a helper — would not match and its operation would drop
// out of the universe silently. Measured against the tree today, all 225
// http.NewRequest calls across both generated clients spell the method as a bare
// literal and none is computed, so the residual is currently empty; it reopens
// only if oapi-codegen changes how it emits the call.
//
// # What the scan matches, and why it is that loose
//
// It walks EVERY selector in a facade body and matches the name against the
// delete operations read out of the generated clients. It deliberately does not
// care what the selector hangs off.
//
// An earlier version cared, and a review wrote a compiling facade that called
// the banned parser and passed anyway. Three shapes walked straight through it:
// a client reached through a receiver field spelled anything other than "ledger"
// or "tracer"; an operation passed as a FUNCTION VALUE rather than called (the
// scan only looked at call targets); and a client hoisted into a local first, so
// the selector hung off a plain identifier. Each escape existed because the scan
// asserted something about the SHAPE of the call rather than about the operation
// being named. Matching the operation name against the generated set is both
// stricter and shorter.
//
// What the scan DOES ask about each mention is whether it is the callee of a
// call, because a mention in any other position cannot be shown to reach the
// seam — see rawMentionOffence.
//
// # Credit attaches to the OCCURRENCE, never to the function
//
// The previous version asked callsAny(fn, "deleteResource"): a FUNCTION-LEVEL
// boolean. A facade that routed one delete through the seam honestly and named a
// second raw delete that never reached it was credited for both, because the
// boolean had already been satisfied somewhere in the body. Its exemption
// bookkeeping had the same shape one level down — rawDecides[raw[0]] recorded
// only the FIRST exempt operation the function named and dropped every other
// one. This is the defect the sibling path-guard scan was carrying when a review
// turned it into a DELETE /v1/rules/{id} leaving with id="..", one scan over;
// live exposure here is zero only because every exempt facade names exactly one
// operation today.
//
// A delete OPERATION CALL is now accounted individually, and every unaccounted
// call is reported, same-named or not.
func TestEveryDeleteRoutesThroughTheSharedSeam(t *testing.T) {
	fset := token.NewFileSet()

	deleteOps := deleteOperations(t, fset)
	require.NotEmpty(t, deleteOps, "found no generated delete operations; the scan is broken, not the code")

	scan := &deleteSeamScan{fset: fset, deleteOps: deleteOps, rawDecides: map[string]bool{}}

	var offenders []string

	for name, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				offenders = append(offenders, packageScopeEscape(fset, name, decl, deleteOps)...)

				continue
			}

			offenders = append(offenders, scan.offences(name, fn)...)
		}
	}

	sort.Strings(offenders)

	require.Empty(t, offenders,
		"every delete must decide success on the status alone, through deleteResource:\n  %s",
		strings.Join(offenders, "\n  "))

	for op := range statusDecidingDeletes {
		require.True(t, scan.rawDecides[op],
			"%s is exempted from deleteResource because it decides success on the status itself, "+
				"so it must hand that call's result to isSuccess; found=%v", op, scan.rawDecides[op])
	}

	// A floor, so a scan that stops matching cannot read as success. 29 today:
	// 27 ledger delete calls through deleteResource plus the two tracer delete
	// calls that decide inline. Adding a delete raises it; losing one fails here.
	//
	// It counts CALL SITES rather than functions, which is the same 29 while every
	// facade names one delete, and the honest number the moment one names two.
	require.GreaterOrEqual(t, scan.seamed, 29,
		"expected the delete seam on at least 29 delete call sites (27 ledger + 2 tracer); found %d", scan.seamed)
}

// deleteSeamScan carries the derived delete universe and the credit the walk
// accumulates: one count per accounted delete CALL, and the exempt operations
// whose inline status decision was actually verified.
type deleteSeamScan struct {
	fset      *token.FileSet
	deleteOps map[string]bool

	seamed     int
	rawDecides map[string]bool
}

// offences reports what one facade function does wrong about deletes, and
// credits what it does right.
//
// Naming a generated PARSER spelling is fatal for the whole function and stops
// there: that spelling IS the broken parser, so there is no seam to look for and
// nothing to credit.
func (s *deleteSeamScan) offences(file string, fn *ast.FuncDecl) []string {
	parsed, raw := deleteMentions(fn.Body, s.deleteOps)
	if len(parsed) == 0 && len(raw) == 0 {
		return nil
	}

	site := " (" + filepath.Base(file) + ":" + strconv.Itoa(s.fset.Position(fn.Pos()).Line) + ")"

	if len(parsed) > 0 {
		return []string{funcLabel(fn) + site + " names the generated parser " +
			strings.Join(s.mentionSites(parsed), ", ") +
			"; a bodiless 204 with a JSON content type fails there"}
	}

	locals := bodyVariables(fn)

	var problems []string

	for _, sel := range raw {
		if problem := s.rawMentionOffence(fn, sel, locals, site); problem != "" {
			problems = append(problems, problem)
		}
	}

	return problems
}

// rawMentionOffence judges ONE mention of a raw delete operation, and returns
// the empty string when that mention is accounted.
//
// Three things have to hold, in order:
//
//  1. the mention is the CALLEE of a call. A mention anywhere else — hoisted
//     into a local, handed to something else as a function value, named and
//     never invoked — produces no response this scan can follow, so it is
//     reported rather than credited. That is the same unknown=flag rule the
//     sibling path-guard scan applies to a delegation it cannot read;
//  2. an operation on the inline-decision list must hand THAT call's result to
//     isSuccess. The exemption is a promise about one call, so it is checked at
//     that call rather than anywhere in the function;
//  3. every other delete call must hand its result to deleteResource.
//
// An exempt operation is deliberately NOT also accepted through deleteResource:
// routing one through the shared seam is the right change, and it comes with
// deleting its entry from statusDecidingDeletes.
func (s *deleteSeamScan) rawMentionOffence(
	fn *ast.FuncDecl,
	sel *ast.SelectorExpr,
	locals map[string]bool,
	site string,
) string {
	op, _ := generatedOperation(sel.Sel.Name)
	where := op + " on line " + strconv.Itoa(s.fset.Position(sel.Pos()).Line)

	call, called := calleeCallOf(fn, sel)
	if !called {
		return funcLabel(fn) + site + " names " + where +
			" without calling it there, so no seam can be verified for that mention"
	}

	if _, exempt := statusDecidingDeletes[op]; exempt {
		if !resultsReach(fn, call, "isSuccess", locals) {
			return funcLabel(fn) + site + " reaches " + where +
				", exempted from deleteResource only because it decides on the status itself," +
				" but that call's result never reaches isSuccess"
		}

		s.rawDecides[op] = true
		s.seamed++

		return ""
	}

	if !resultsReach(fn, call, "deleteResource", locals) {
		return funcLabel(fn) + site + " reaches " + where + " without deleteResource"
	}

	s.seamed++

	return ""
}

// mentionSites names each mention by operation AND line, because one function
// can name the same operation several times and the reader needs the occurrence,
// not the name.
func (s *deleteSeamScan) mentionSites(mentions []*ast.SelectorExpr) []string {
	named := make([]string, 0, len(mentions))

	for _, sel := range mentions {
		op, _ := generatedOperation(sel.Sel.Name)
		named = append(named, op+" on line "+strconv.Itoa(s.fset.Position(sel.Pos()).Line))
	}

	return named
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
	parsed, raw := deleteMentions(decl, deleteOps)
	if len(parsed) == 0 && len(raw) == 0 {
		return nil
	}

	return []string{"the package-level declaration at " + filepath.Base(path) + ":" +
		strconv.Itoa(fset.Position(decl.Pos()).Line) + " names " +
		strings.Join(mentionNames(append(parsed, raw...)), ", ") +
		"; a generated delete reached from package scope leaves every caller naming a bare " +
		"identifier, which no scan of function bodies can attribute to a delete"}
}

// statusDecidingDeletes are the deletes that do NOT route through deleteResource
// because they decide success on the status inline, the same way it does. Both
// are tracer-plane and both predate the seam.
//
// Literal rather than derived: a derived exemption would quietly absorb the next
// delete someone writes by hand. Each entry costs a reason.
//
// The exemption is per OPERATION and it is checked per CALL. A function mixing
// an exempt delete with a real one used to fall out of the exemption entirely,
// which was the only defence available while credit was function-level; now each
// call is judged against its own operation, so the mix needs no special case.
var statusDecidingDeletes = map[string]string{
	"DeleteRule":  "tracer-plane; decides on isSuccess inline (rules_facade.go)",
	"DeleteLimit": "tracer-plane; decides on isSuccess inline (limits_facade.go)",
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
// that issues DELETE, through the one scan collectWriteOperations also uses —
// see operationsMatchingMethod for why the two universes must not each keep
// their own copy of it.
func collectDeleteOperations(t *testing.T, path string, file *ast.File, ops map[string]bool) {
	t.Helper()

	operationsMatchingMethod(t, path, file, deleteMethod, ops)
}

// deleteMentions splits the mentions of a generated delete operation inside a
// node into the ones reached through the generated parser (*WithResponse) and
// the ones reached raw — one entry per OCCURRENCE, not one per operation.
//
// Returning NODES rather than names is what lets the caller judge each mention
// where it sits. A set keyed by operation name collapses every distinct mention
// of one operation into a single entry, which is exactly how one honest delete
// used to launder every other mention of that name in the same function.
//
// It takes any ast.Node — a function body, or a whole package-level declaration
// — so the same matching covers both scopes. It inspects SelectorExpr nodes
// rather than call targets, and asks nothing about the receiver. A generated
// delete operation cannot be NAMED for any innocent reason, so naming one is the
// whole signal — whether it is called on f.ledger, on a hoisted local, on a
// differently-spelled field, handed to something else as a function value, or
// bound to a package-level var.
func deleteMentions(node ast.Node, deleteOps map[string]bool) (parsed, raw []*ast.SelectorExpr) {
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		op, ok := generatedOperation(sel.Sel.Name)
		if !ok || !deleteOps[op] {
			return true
		}

		if isParserSpelling(sel.Sel.Name) {
			parsed = append(parsed, sel)
		} else {
			raw = append(raw, sel)
		}

		return true
	})

	return parsed, raw
}

// mentionNames reduces mentions to the sorted set of operations they name, for
// the one report that describes a whole DECLARATION rather than a call site.
func mentionNames(mentions []*ast.SelectorExpr) []string {
	seen := map[string]bool{}

	for _, sel := range mentions {
		if op, ok := generatedOperation(sel.Sel.Name); ok {
			seen[op] = true
		}
	}

	return sortedKeys(seen)
}

// calleeCallOf returns the call whose callee is sel, which is the only position
// where a mention is an OPERATION CALL rather than a value handed elsewhere.
//
// Parentheses are read through for the reason the sibling scan gives: they carry
// no meaning, and refusing (f.ledger.DeleteAsset)(ctx, ...) would be a pure false
// positive. Nothing else is read through.
func calleeCallOf(fn *ast.FuncDecl, sel *ast.SelectorExpr) (*ast.CallExpr, bool) {
	var found *ast.CallExpr

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if callee, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok && callee == sel {
			found = call

			return false
		}

		return true
	})

	return found, found != nil
}

// resultsReach reports whether the results of one call reach a package-local
// function named seam.
//
// Two positions count, and both are properties of THIS call rather than of the
// function around it:
//
//   - the call is a direct argument of a seam call, so its results ARE the seam
//     call's arguments; or
//   - the assignment most tightly enclosing the call binds its results to body
//     variables, and the seam call names at least one of them.
//
// # What this matcher can and cannot see
//
// It is source-structural. There is no type information and no dataflow: it
// matches an assignment's left-hand names against the identifiers a seam call
// mentions, and it cannot prove that the variable it matched is the one carrying
// the response. A function whose delete result is stored in a struct field, sent
// over a channel, or returned by a local wrapper and only THEN handed to the
// seam links through none of that and is reported.
//
// That is the deliberate direction. The narrowest link that covers every live
// shape is "same statement, same variable", and everything looser is a way to
// credit a call whose response nobody looked at. Both live shapes clear it: the
// 27 ledger deletes bind resp, err and hand exactly those to deleteResource, and
// the two tracer deletes are nested one level inside readRawResponse, whose
// enclosing assignment binds the resp that isSuccess then reads. The cost is a
// false positive on an honest delete written some third way, whose fix is to
// write it the way the other 29 are.
func resultsReach(fn *ast.FuncDecl, call *ast.CallExpr, seam string, locals map[string]bool) bool {
	bound := resultVariables(fn, call, locals)

	for _, seamCall := range localCalls(fn, seam) {
		for _, arg := range seamCall.Args {
			if inner, ok := ast.Unparen(arg).(*ast.CallExpr); ok && inner == call {
				return true
			}
		}

		for name := range identifiersIn(seamCall.Args) {
			if bound[name] {
				return true
			}
		}
	}

	return false
}

// resultVariables returns the body variables that the assignment most tightly
// enclosing call binds its results to.
//
// Tightest wins by SPAN, because an assignment can sit inside another one's
// right-hand side through a function literal, and the inner binding is the one
// that receives these results.
func resultVariables(fn *ast.FuncDecl, call *ast.CallExpr, locals map[string]bool) map[string]bool {
	var best *ast.AssignStmt

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || !assignsResultOf(assign, call) {
			return true
		}

		if best == nil || assign.End()-assign.Pos() < best.End()-best.Pos() {
			best = assign
		}

		return true
	})

	if best == nil {
		return nil
	}

	bound := map[string]bool{}

	for _, lhs := range best.Lhs {
		if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok && locals[ident.Name] {
			bound[ident.Name] = true
		}
	}

	return bound
}

// assignsResultOf reports whether call sits inside one of the assignment's
// right-hand expressions.
func assignsResultOf(assign *ast.AssignStmt, call *ast.CallExpr) bool {
	for _, rhs := range assign.Rhs {
		if call.Pos() >= rhs.Pos() && call.End() <= rhs.End() {
			return true
		}
	}

	return false
}

// localCalls returns the calls fn makes to a package-local function named name.
func localCalls(fn *ast.FuncDecl, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if callee, ok := calleeName(call.Fun); ok && callee == name {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

// bodyVariables returns the VARIABLES declared inside the function body.
//
// Parameters and the receiver are excluded by construction — they are not in the
// body — and that is the point: every call in a function shares its parameters,
// so a parameter appearing on both sides of a comparison is evidence of nothing.
// Body CONSTANTS are excluded for the same reason one step further: a constant is
// a compile-time value that cannot carry a response or a resolved key, so the
// operation label every facade declares (const operation = "Assets.Delete") must
// not be able to link a call to anything.
func bodyVariables(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}

	collectShortDeclarations(fn, names)
	collectVarDeclarations(fn, names)

	return names
}

// collectShortDeclarations records the names bound by := inside the body.
func collectShortDeclarations(fn *ast.FuncDecl, names map[string]bool) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE {
			return true
		}

		for _, lhs := range assign.Lhs {
			if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok {
				names[ident.Name] = true
			}
		}

		return true
	})
}

// collectVarDeclarations records the names bound by a var declaration inside the
// body. const is deliberately absent — see bodyVariables.
func collectVarDeclarations(fn *ast.FuncDecl, names map[string]bool) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			return true
		}

		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for _, ident := range value.Names {
				names[ident.Name] = true
			}
		}

		return true
	})
}

// identifiersIn returns every identifier named anywhere inside the expressions,
// nesting included: a value can reach a call buried in a composite literal or in
// another call's arguments, and the generated V2 creates do exactly that.
func identifiersIn(exprs []ast.Expr) map[string]bool {
	names := map[string]bool{}

	for _, expr := range exprs {
		ast.Inspect(expr, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				names[ident.Name] = true
			}

			return true
		})
	}

	return names
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
