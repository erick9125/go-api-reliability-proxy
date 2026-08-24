package rules

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatcherMethodsAndFirstMatch(t *testing.T) {
	matcher := NewMatcher([]Rule{
		{
			Name: "payments-post",
			Match: MatchConfig{
				Path:    "/payments/*",
				Methods: []string{"POST"},
			},
		},
		{
			Name: "users-all",
			Match: MatchConfig{
				Path: "/users/*",
			},
		},
		{
			Name: "users-get-later",
			Match: MatchConfig{
				Path:    "/users/*",
				Methods: []string{"GET"},
			},
		},
	})

	tests := []struct {
		name         string
		method       string
		path         string
		expectedRule string
		wantNil      bool
	}{
		{name: "GET does not match POST rule", method: http.MethodGet, path: "/payments/1", wantNil: true},
		{name: "POST matches POST", method: http.MethodPost, path: "/payments/1", expectedRule: "payments-post"},
		{name: "no methods means all", method: http.MethodDelete, path: "/users/42", expectedRule: "users-all"},
		{name: "first matching rule wins", method: http.MethodGet, path: "/users/42", expectedRule: "users-all"},
		{name: "different path", method: http.MethodGet, path: "/inventory/1", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			got := matcher.Match(req)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected no match, got %q", got.Name)
				}
				return
			}
			if got == nil {
				t.Fatal("expected a match")
			}
			if got.Name != tt.expectedRule {
				t.Fatalf("got rule %q, want %q", got.Name, tt.expectedRule)
			}
		})
	}
}

func TestMatcherNormalizesRequestMethod(t *testing.T) {
	matcher := NewMatcher([]Rule{{
		Name: "only-get",
		Match: MatchConfig{
			Path:    "/health",
			Methods: []string{"GET"},
		},
	}})
	req := httptest.NewRequest("get", "/health", nil)
	got := matcher.Match(req)
	if got == nil {
		t.Fatal("expected method to match after uppercasing")
	}
}

func BenchmarkMatcher(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("%d_rules", n), func(b *testing.B) {
			rs := make([]Rule, n)
			for i := 0; i < n; i++ {
				rs[i] = Rule{
					Name: fmt.Sprintf("rule-%d", i),
					Match: MatchConfig{
						Path: fmt.Sprintf("/p/%d/*", i),
					},
				}
			}
			matcher := NewMatcher(rs)
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/p/%d/item", n-1), nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = matcher.Match(req)
			}
		})
	}
}
