package metrics

import "sync/atomic"

type Metrics struct {
	requests       atomic.Uint64
	matched        atomic.Uint64
	faultsInjected atomic.Uint64
	proxied        atomic.Uint64
}

func New() *Metrics {
	return &Metrics{}
}

func (m *Metrics) RecordRequest() {
	m.requests.Add(1)
}

func (m *Metrics) RecordMatch() {
	m.matched.Add(1)
}

func (m *Metrics) RecordFault() {
	m.faultsInjected.Add(1)
}

func (m *Metrics) RecordProxied() {
	m.proxied.Add(1)
}

func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		Requests:       m.requests.Load(),
		Matched:        m.matched.Load(),
		FaultsInjected: m.faultsInjected.Load(),
		Proxied:        m.proxied.Load(),
	}
}
