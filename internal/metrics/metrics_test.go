package metrics

import "testing"

func TestMetricsSnapshot(t *testing.T) {
	m := New()
	m.RecordRequest()
	m.RecordRequest()
	m.RecordMatch()
	m.RecordFault()
	m.RecordProxied()

	snap := m.Snapshot()
	if snap.Requests != 2 || snap.Matched != 1 || snap.FaultsInjected != 1 || snap.Proxied != 1 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}
