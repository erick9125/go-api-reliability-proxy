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
			got, ok := matcher.Match(req)
			if tt.wantNil {
				if ok {
					t.Fatalf("expected no match, got %q", got.Name)
				}
				return
			}
			if !ok {
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
	if _, ok := matcher.Match(req); !ok {
		t.Fatal("expected method to match after uppercasing")
	}
}

// Mutating the caller's slice must not change what later requests match.
func TestMatcherOwnsItsRules(t *testing.T) {
	probability := 0.5
	source := []Rule{{
		Name:  "flaky",
		Match: MatchConfig{Path: "/a", Methods: []string{"GET"}},
		Effects: Effects{
			Failure: &FailureConfig{
				Probability: &probability,
				Status:      503,
				Headers:     map[string]HeaderValues{"X-Fault": {"yes"}},
			},
		},
	}}
	matcher := NewMatcher(source)

	source[0].Name = "mutated"
	source[0].Match.Methods[0] = "POST"
	*source[0].Effects.Failure.Probability = 1
	source[0].Effects.Failure.Status = 500
	source[0].Effects.Failure.Headers["X-Fault"] = HeaderValues{"no"}

	req := httptest.NewRequest(http.MethodGet, "/a", nil)
	got, ok := matcher.Match(req)
	if !ok {
		t.Fatal("expected a match after the source slice was mutated")
	}
	if got.Name != "flaky" {
		t.Errorf("name = %q, want the value captured at construction", got.Name)
	}
	if *got.Effects.Failure.Probability != 0.5 {
		t.Errorf("probability = %v, want 0.5", *got.Effects.Failure.Probability)
	}
	if got.Effects.Failure.Status != 503 {
		t.Errorf("status = %d, want 503", got.Effects.Failure.Status)
	}
	if v := got.Effects.Failure.Headers["X-Fault"][0]; v != "yes" {
		t.Errorf("header = %q, want %q", v, "yes")
	}

	// Assigning to the returned value must not reach the matcher; Effects
	// pointers are documented as shared, so they are outside this guarantee.
	got.Name = "local edit"
	again, _ := matcher.Match(req)
	if again.Name != "flaky" {
		t.Errorf("name = %q after editing a returned rule, want %q", again.Name, "flaky")
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
				_, _ = matcher.Match(req)
			}
		})
	}
}
