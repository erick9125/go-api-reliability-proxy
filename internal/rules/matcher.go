package rules

import (
	"net/http"
	"strings"
)

type Matcher interface {
	// Match reports the first rule matching the request, if any.
	//
	// The rule is returned by value, so the caller cannot reach the matcher's
	// slice. The pointers inside Effects are still shared with the matcher, so
	// the result must be treated as read-only: it is handed to every request
	// that matches the same rule, concurrently. Cloning per call was rejected
	// because it would allocate on every proxied request to guard against a
	// mutation nothing performs.
	Match(*http.Request) (Rule, bool)
}

type RuleMatcher struct {
	rules []Rule
}

// NewMatcher takes a private deep copy of the rules, so a caller that keeps
// mutating its own slice cannot race with requests being served. The matcher is
// immutable once built.
func NewMatcher(rules []Rule) *RuleMatcher {
	copied := make([]Rule, len(rules))
	for i := range rules {
		copied[i] = cloneRule(rules[i])
	}
	return &RuleMatcher{rules: copied}
}

func (m *RuleMatcher) Match(r *http.Request) (Rule, bool) {
	method := strings.ToUpper(r.Method)
	path := r.URL.Path
	for i := range m.rules {
		rule := m.rules[i]
		if !matchMethod(rule.Match.Methods, method) {
			continue
		}
		if !MatchPath(rule.Match.Path, path) {
			continue
		}
		return rule, true
	}
	return Rule{}, false
}

func matchMethod(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, candidate := range methods {
		if candidate == method {
			return true
		}
	}
	return false
}
