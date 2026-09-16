package authz

import (
	"errors"
	"fmt"
	"strings"
)

// ErrScopeSyntax is returned by ParseScope (and ValidateForMint) for a scope
// string outside the grammar.
var ErrScopeSyntax = errors.New("authz: scope syntax")

// Scope grammar (design §3.1):
//
//	scope      := action-pat [ ":" selector ]
//	action-pat := "*" | action | family ".*"     family = dotted prefix of an action
//	selector   := "*" | segment { ":" segment }  "*" alone = match anything
//	segment    := "*" | 1*( ALPHA / DIGIT / "-" / "_" / "." / " " )   no ":" inside a segment
//
//   - Lower-case ASCII only (ParseScope rejects upper-case; store scopes through
//     CanonScopes). Want selectors are canonicalised before comparison.
//   - "*" as the whole trailing selector matches every selector including the
//     empty one; a "*" segment matches exactly one segment (segment counts must
//     be equal), so "room.join:host:*:game_filtered" matches
//     "room.join:host:box1:game_filtered" but not the bare "room.join:host:box1".
//   - An exact action-pat with no selector matches only the empty selector
//     ("library.manage" ≠ "library.manage:iso1"); a family pattern or bare "*"
//     with no selector matches every selector ("lan.*" covers
//     "lan.saves.file:gametype/abc").

// ScopePat is a parsed scope pattern.
type ScopePat struct {
	Action   string   // exact action, or the family for Family patterns; "" when Any
	Family   bool     // action-pat was "<family>.*"
	Selector []string // selector segments; nil when the scope has no selector
	Any      bool     // the bare "*" scope: every action, every selector
}

// wildcardSegment is the segment / selector wildcard.
const wildcardSegment = "*"

// Canon canonicalises a scope string: trim, lower-case, and collapse internal
// whitespace inside each ":"-separated segment. It never validates —
// ParseScope does — so Canon(bad) is still bad.
func Canon(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	segs := strings.Split(s, ":")
	for i, seg := range segs {
		segs[i] = canonSegment(seg)
	}
	return strings.Join(segs, ":")
}

// canonSegment is the per-segment fold Canon and parseWant apply: lower-case
// with internal whitespace collapsed. The predicates that compare an instance
// name against a principal's binding or owned box use the same fold so a
// literal compare can never disagree with the scope match that admitted the
// principal.
func canonSegment(seg string) string {
	return strings.Join(strings.Fields(strings.ToLower(seg)), " ")
}

// ParseScope parses a scope pattern. ErrScopeSyntax on: empty, leading or
// trailing ':', an empty segment, "*" that is not a whole segment, a segment
// character outside the grammar, upper-case anywhere, or an action-pat that is
// neither a known Action, "<family>.*" for a family that prefixes at least one
// known action, nor the bare "*".
func ParseScope(s string) (ScopePat, error) {
	if s == "" {
		return ScopePat{}, fmt.Errorf("%w: empty scope", ErrScopeSyntax)
	}
	if strings.ToLower(s) != s {
		return ScopePat{}, fmt.Errorf("%w: %q must be lower-case", ErrScopeSyntax, s)
	}
	if strings.HasPrefix(s, ":") || strings.HasSuffix(s, ":") {
		return ScopePat{}, fmt.Errorf("%w: %q has a leading/trailing ':'", ErrScopeSyntax, s)
	}

	actionPat, selector, hasSelector := strings.Cut(s, ":")

	var pat ScopePat
	switch {
	case actionPat == wildcardSegment:
		if hasSelector {
			return ScopePat{}, fmt.Errorf("%w: %q — bare \"*\" takes no selector", ErrScopeSyntax, s)
		}
		pat.Any = true
	case strings.HasSuffix(actionPat, ".*"):
		family := strings.TrimSuffix(actionPat, ".*")
		if !isActionFamily(family) {
			return ScopePat{}, fmt.Errorf("%w: %q is not an action family", ErrScopeSyntax, family)
		}
		pat.Action = family
		pat.Family = true
	default:
		if !Action(actionPat).Known() {
			return ScopePat{}, fmt.Errorf("%w: %q is not an action", ErrScopeSyntax, actionPat)
		}
		pat.Action = actionPat
	}

	if !hasSelector {
		return pat, nil
	}
	segs := strings.Split(selector, ":")
	for _, seg := range segs {
		if err := validateSegment(seg); err != nil {
			return ScopePat{}, fmt.Errorf("%w: %q: %v", ErrScopeSyntax, s, err)
		}
	}
	pat.Selector = segs
	return pat, nil
}

// validateSegment enforces the segment production of the grammar.
func validateSegment(seg string) error {
	if seg == "" {
		return errors.New("empty segment")
	}
	if seg == wildcardSegment {
		return nil
	}
	for _, r := range seg {
		switch {
		case r == '*':
			return errors.New("\"*\" must be a whole segment")
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.', r == ' ':
		default:
			return fmt.Errorf("character %q not allowed in a segment", r)
		}
	}
	return nil
}

// parseWant splits a want string ("<action>" or "<action>:<selector>") into
// its action and canonicalised selector segments. ok is false when the action
// is unknown. Want selectors are not validated against the segment grammar —
// they are resource identifiers (LAN file ids contain "/", instance names have
// no charset rule) — only canonicalised, and "*" in a want is a literal.
func parseWant(want string) (action Action, selector []string, ok bool) {
	head, rest, hasSelector := strings.Cut(want, ":")
	action = Action(head)
	if !action.Known() {
		return "", nil, false
	}
	if !hasSelector {
		return action, nil, true
	}
	selector = strings.Split(rest, ":")
	for i, seg := range selector {
		selector[i] = canonSegment(seg)
	}
	return action, selector, true
}

// Match reports whether scope (a pattern) grants want ("<action>" or
// "<action>:<selector>"). False on any parse error of either side.
func Match(scope, want string) bool {
	pat, err := ParseScope(scope)
	if err != nil {
		return false
	}
	action, selector, ok := parseWant(want)
	if !ok {
		return false
	}
	return pat.matchAction(action) && pat.matchSelector(selector)
}

// matchAction applies the action-pat part of the pattern.
func (pat ScopePat) matchAction(a Action) bool {
	switch {
	case pat.Any:
		return true
	case pat.Family:
		return strings.HasPrefix(string(a), pat.Action+".")
	default:
		return string(a) == pat.Action
	}
}

// matchSelector applies the selector part of the pattern to the want's
// canonicalised segments.
func (pat ScopePat) matchSelector(want []string) bool {
	if len(pat.Selector) == 0 {
		// No selector: exact actions cover only the empty selector; family /
		// bare-"*" patterns cover every selector.
		return pat.Family || pat.Any || len(want) == 0
	}
	if len(pat.Selector) == 1 && pat.Selector[0] == wildcardSegment {
		return true
	}
	if len(pat.Selector) != len(want) {
		return false
	}
	for i, seg := range pat.Selector {
		if seg != wildcardSegment && seg != want[i] {
			return false
		}
	}
	return true
}

// MatchAny returns the first scope in scopes that matches want.
func MatchAny(scopes []string, want string) (matched string, ok bool) {
	for _, s := range scopes {
		if Match(s, want) {
			return s, true
		}
	}
	return "", false
}

// ScopeFor is the want string for (a, r): string(a) + ":" + r.Selector()
// when the selector is non-empty, else just the action.
func ScopeFor(a Action, r Resource) string {
	if sel := r.Selector(); sel != "" {
		return string(a) + ":" + sel
	}
	return string(a)
}

// IsWildcard reports whether the scope is "*", "<family>.*", has the selector
// "*", or has any "*" segment. Unparseable scopes are not wildcards (false).
func IsWildcard(s string) bool {
	pat, err := ParseScope(s)
	if err != nil {
		return false
	}
	return pat.isWildcard()
}

func (pat ScopePat) isWildcard() bool {
	if pat.Any || pat.Family {
		return true
	}
	for _, seg := range pat.Selector {
		if seg == wildcardSegment {
			return true
		}
	}
	return false
}

// NamesOneInstance reports whether the scope's selector names exactly one
// literal instance: a single literal segment, or "host:<inst>[:<class>]" with
// a literal <inst> (the class may be a wildcard; the host aggregates
// host:all / host:summary are not instances). Unparseable ⇒ false.
func NamesOneInstance(s string) bool {
	pat, err := ParseScope(s)
	if err != nil {
		return false
	}
	_, ok := pat.namedInstance()
	return ok
}

// namedInstance returns the single literal instance the selector names.
func (pat ScopePat) namedInstance() (string, bool) {
	switch len(pat.Selector) {
	case 1:
		if pat.Selector[0] == wildcardSegment {
			return "", false
		}
		return pat.Selector[0], true
	case 2, 3:
		if pat.Selector[0] != hostRoomType {
			return "", false
		}
		inst := pat.Selector[1]
		if inst == wildcardSegment || hostAggregateInstances[inst] {
			return "", false
		}
		return inst, true
	default:
		return "", false
	}
}
