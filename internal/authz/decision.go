package authz

import "reflect"

// Decision is the outcome of CanWith. Reason is stable and log-safe:
//
//	"superuser"        superuser allow-all
//	"internal"         internal allow-all
//	"scope:<s>"        the scope s matched (also in Matched)
//	"pred:<name>"      the predicate's verdict, allow or deny
//	"deny:<name>"      a Deny predicate fired (last_admin)
//	"kind"             principal kind not admitted by the rule
//	"unknown_action"   action outside the closed vocabulary
//	"no_scope"         no scope matched and the cell has no predicate to fall back on
//	"no_pred"          the predicate denied without naming itself
//	"resource"         resource kind / shape not accepted by the rule
//	"no_deps"          nil Deps (never a panic, always a deny)
type Decision struct {
	Allow   bool
	Reason  string
	Matched string // the scope string that matched, when Reason starts with "scope:"
}

// Reason strings (see Decision).
const (
	reasonSuperuser     = "superuser"
	reasonInternal      = "internal"
	reasonKind          = "kind"
	reasonUnknownAction = "unknown_action"
	reasonNoScope       = "no_scope"
	reasonNoPred        = "no_pred"
	reasonResource      = "resource"
	reasonNoDeps        = "no_deps"
)

func deny(reason string) Decision { return Decision{Reason: reason} }

func allow(reason string) Decision { return Decision{Allow: true, Reason: reason} }

// Can reports whether p may perform a on r. See CanWith for the order.
func Can(deps Deps, p Principal, a Action, r Resource) bool {
	return CanWith(deps, p, a, r).Allow
}

// CanWith evaluates the §4 rule for (a, r) against p in a fixed order
// (design §2.4, pinned by TestCanOrder):
//
//  1. rule lookup — unknown action ⇒ deny "unknown_action"; a resource the
//     rule does not accept ⇒ deny "resource"; nil deps ⇒ deny "no_deps"
//  2. the rule's Deny predicates, for every kind including superuser
//     (last_admin; internal is exempt inside the predicate, §4.4)
//  3. superuser ⇒ allow "superuser"; internal ⇒ allow "internal"
//  4. kind ∉ rule kinds ⇒ deny "kind"
//  5. when the cell takes scopes, the first p.Scopes match on the rule's
//     wants (ScopeFor(a, r) unless overridden) ⇒ allow "scope:<s>" (S, S ∨ P),
//     or unlocks the predicate (S ∧ P); no match under S ∧ P ⇒ deny "no_scope"
//  6. the cell's predicate ⇒ its verdict "pred:<name>"
//  7. otherwise deny "no_scope"
//
// It never panics: a nil or typed-nil deps, a zero Principal or a zero
// Resource all produce a deny.
func CanWith(deps Deps, p Principal, a Action, r Resource) Decision {
	rl, ok := rules[a]
	if !ok {
		return deny(reasonUnknownAction)
	}
	if !rl.accepts(r) {
		return deny(reasonResource)
	}
	if nilDeps(deps) {
		return deny(reasonNoDeps)
	}

	for _, d := range rl.Deny {
		if bad, name := d(deps, p, r); bad {
			return deny("deny:" + name)
		}
	}

	switch p.Kind {
	case KindSuperuser:
		return allow(reasonSuperuser)
	case KindInternal:
		return allow(reasonInternal)
	}

	cell, ok := rl.kindsFor(r)[p.Kind]
	if !ok {
		return deny(reasonKind)
	}

	matched, scopeOK := "", false
	if cell.Mode != modePred {
		for _, want := range rl.wantsFor(a, r) {
			if s, ok := MatchAny(p.Scopes, want); ok {
				matched, scopeOK = s, true
				break
			}
		}
	}

	switch cell.Mode {
	case modeScope:
		if scopeOK {
			return Decision{Allow: true, Reason: "scope:" + matched, Matched: matched}
		}
		return deny(reasonNoScope)
	case modeScopeOrPred:
		if scopeOK {
			return Decision{Allow: true, Reason: "scope:" + matched, Matched: matched}
		}
		return verdict(cell.Pred, deps, p, r)
	case modeScopeAndPred:
		if !scopeOK {
			return deny(reasonNoScope)
		}
		return verdict(cell.Pred, deps, p, r)
	case modePred:
		return verdict(cell.Pred, deps, p, r)
	}
	return deny(reasonNoScope)
}

// verdict turns a predicate's answer into a Decision.
func verdict(pred predicate, deps Deps, p Principal, r Resource) Decision {
	if pred == nil {
		return deny(reasonNoPred)
	}
	ok, name := pred(deps, p, r)
	if name == "" {
		// A nameless verdict is a table bug; fail closed either way.
		return deny(reasonNoPred)
	}
	if ok {
		return allow("pred:" + name)
	}
	return deny("pred:" + name)
}

// nilDeps reports whether deps is nil or a typed-nil pointer / map / func
// wrapped in the interface (calling through it would panic).
func nilDeps(deps Deps) bool {
	if deps == nil {
		return true
	}
	v := reflect.ValueOf(deps)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Func, reflect.Interface, reflect.Slice, reflect.Chan:
		return v.IsNil()
	}
	return false
}
