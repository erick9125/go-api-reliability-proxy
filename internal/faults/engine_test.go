package faults

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/erick9125/go-api-reliability-proxy/internal/metrics"
	"github.com/erick9125/go-api-reliability-proxy/internal/rules"
)

func TestProbabilityInjection(t *testing.T) {
	tests := []struct {
		name        string
		random      float64
		probability float64
		wantStop    bool
		wantStatus  int
	}{
		{name: "inject when random below probability", random: 0.10, probability: 0.25, wantStop: true, wantStatus: 503},
		{name: "passthrough when random above probability", random: 0.80, probability: 0.25, wantStop: false, wantStatus: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := New(Options{
				Random:  FixedRandom{Value: tt.random},
				Sleeper: &recordingSleeper{},
				Metrics: metrics.New(),
				Logger:  silentLogger(),
			})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/inventory/1", nil)
			prob := tt.probability
			result, err := engine.Apply(req.Context(), rules.Rule{
				Name: "inventory-failures",
				Effects: rules.Effects{
					Failure: &rules.FailureConfig{
						Probability: &prob,
						Status:      http.StatusServiceUnavailable,
					},
				},
			}, rec, req)
			if err != nil {
				t.Fatal(err)
			}
			if result.Stop != tt.wantStop {
				t.Fatalf("Stop=%v, want %v", result.Stop, tt.wantStop)
			}
			if tt.wantStop && rec.Code != tt.wantStatus {
				t.Fatalf("status=%d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestLatencyUsesSleeperAndContinues(t *testing.T) {
	sleeper := &recordingSleeper{}
	engine := New(Options{
		Random:  FixedRandom{Value: 0},
		Sleeper: sleeper,
		Metrics: metrics.New(),
		Logger:  silentLogger(),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/payments", nil)
	fixed := rules.Duration{Duration: 1200 * time.Millisecond}
	result, err := engine.Apply(req.Context(), rules.Rule{
		Name: "payments-latency",
		Effects: rules.Effects{
			Latency: &rules.LatencyConfig{Fixed: &fixed},
		},
	}, rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stop {
		t.Fatal("latency should not stop proxying")
	}
	if sleeper.calls != 1 {
		t.Fatalf("sleeper calls=%d", sleeper.calls)
	}
	if sleeper.delay != 1200*time.Millisecond {
		t.Fatalf("delay=%s", sleeper.delay)
	}
}

func TestRandomLatencyRange(t *testing.T) {
	sleeper := &recordingSleeper{}
	engine := New(Options{
		Random:  FixedRandom{Value: 0.5},
		Sleeper: sleeper,
		Metrics: metrics.New(),
		Logger:  silentLogger(),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/jitter", nil)
	min := rules.Duration{Duration: 100 * time.Millisecond}
	max := rules.Duration{Duration: 300 * time.Millisecond}
	_, err := engine.Apply(req.Context(), rules.Rule{
		Name: "jitter",
		Effects: rules.Effects{
			Latency: &rules.LatencyConfig{Min: &min, Max: &max},
		},
	}, rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if sleeper.delay != 200*time.Millisecond {
		t.Fatalf("delay=%s, want 200ms", sleeper.delay)
	}
}

func TestLatencyCancellation(t *testing.T) {
	engine := New(Options{
		Random:  FixedRandom{Value: 0},
		Sleeper: TimerSleeper{},
		Metrics: metrics.New(),
		Logger:  silentLogger(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(ctx)
	fixed := rules.Duration{Duration: time.Second}
	result, err := engine.Apply(ctx, rules.Rule{
		Name: "slow",
		Effects: rules.Effects{
			Latency: &rules.LatencyConfig{Fixed: &fixed},
		},
	}, rec, req)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !result.Stop {
		t.Fatal("expected stop after cancellation")
	}
}

func TestSyntheticResponseHeadersAndBody(t *testing.T) {
	engine := New(Options{
		Random:  FixedRandom{Value: 1},
		Sleeper: &recordingSleeper{},
		Metrics: metrics.New(),
		Logger:  silentLogger(),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
	result, err := engine.Apply(req.Context(), rules.Rule{
		Name: "rate-limit-users",
		Effects: rules.Effects{
			Response: &rules.ResponseConfig{
				Status:  http.StatusTooManyRequests,
				Headers: map[string]string{"Retry-After": "10", "Content-Type": "application/json"},
				Body:    `{"error":"rate limited"}`,
			},
		},
	}, rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stop {
		t.Fatal("synthetic response should stop proxying")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "10" {
		t.Fatalf("Retry-After=%q", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type=%q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"error":"rate limited"}` {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestTimeoutReturns504(t *testing.T) {
	engine := New(Options{
		Random:  FixedRandom{Value: 0},
		Sleeper: &recordingSleeper{},
		Metrics: metrics.New(),
		Logger:  silentLogger(),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hang", nil)
	result, err := engine.Apply(req.Context(), rules.Rule{
		Name: "hang",
		Effects: rules.Effects{
			Timeout: &rules.TimeoutConfig{Duration: rules.Duration{Duration: 30 * time.Second}},
		},
	}, rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stop {
		t.Fatal("timeout should stop")
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status=%d", rec.Code)
	}
}

type recordingSleeper struct {
	delay time.Duration
	calls int
}

func (s *recordingSleeper) Sleep(ctx context.Context, d time.Duration) error {
	s.calls++
	s.delay = d
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
