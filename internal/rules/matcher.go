package rules

import (
	"net/http"
	"strings"
)

type Matcher interface {
	Match(*http.Request) *Rule
}

type RuleMatcher struct {
	rules []Rule
}

func NewMatcher(rules []Rule) *RuleMatcher {
	copied := make([]Rule, len(rules))
	copy(copied, rules)
	return &RuleMatcher{rules: copied}
}

func (m *RuleMatcher) Match(r *http.Request) *Rule {
	method := strings.ToUpper(r.Method)
	path := r.URL.Path
	for i := range m.rules {
		rule := &m.rules[i]
		if !matchMethod(rule.Match.Methods, method) {
			continue
		}
		if !MatchPath(rule.Match.Path, path) {
			continue
		}
		return rule
	}
	return nil
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
