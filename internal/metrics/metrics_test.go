package metrics

import "testing"

func TestMetricsSnapshot(t *testing.T) {
	m := New()
	m.RecordRequest()
	m.RecordRequest()
	m.RecordMatch()
	// One request, two effects.
	m.RecordFault()
	m.RecordFault()
	m.RecordRequestFaulted()
	m.RecordProxied()

	snap := m.Snapshot()
	if snap.Requests != 2 || snap.Matched != 1 || snap.Proxied != 1 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.FaultsInjected != 2 {
		t.Fatalf("faultsInjected = %d, want 2", snap.FaultsInjected)
	}
	if snap.RequestsFaulted != 1 {
		t.Fatalf("requestsFaulted = %d, want 1", snap.RequestsFaulted)
	}
}
