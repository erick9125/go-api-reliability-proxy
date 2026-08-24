package rules

import "testing"

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{name: "exact match", pattern: "/users", path: "/users", expected: true},
		{name: "exact mismatch", pattern: "/users", path: "/users/42", expected: false},
		{name: "exact different path", pattern: "/users", path: "/orders", expected: false},
		{name: "prefix wildcard child", pattern: "/users/*", path: "/users/42", expected: true},
		{name: "prefix wildcard nested", pattern: "/users/*", path: "/users/42/orders", expected: true},
		{name: "prefix wildcard base", pattern: "/users/*", path: "/users", expected: true},
		{name: "prefix wildcard trailing slash", pattern: "/users/*", path: "/users/", expected: true},
		{name: "prefix does not match sibling", pattern: "/users/*", path: "/usersX", expected: false},
		{name: "prefix does not match parent", pattern: "/users/*", path: "/user", expected: false},
		{name: "root wildcard", pattern: "/*", path: "/anything", expected: true},
		{name: "root wildcard exact root", pattern: "/*", path: "/", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchPath(tt.pattern, tt.path)
			if got != tt.expected {
				t.Fatalf("MatchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.expected)
			}
		})
	}
}
