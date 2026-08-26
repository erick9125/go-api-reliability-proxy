package metrics

import "sync/atomic"

type Metrics struct {
	requests        atomic.Uint64
	matched         atomic.Uint64
	faultsInjected  atomic.Uint64
	requestsFaulted atomic.Uint64
	proxied         atomic.Uint64
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

// RecordFault counts one effect taking hold. A single request can raise it more
// than once, because a rule may combine effects such as latency and failure.
func (m *Metrics) RecordFault() {
	m.faultsInjected.Add(1)
}

// RecordRequestFaulted counts one request that experienced at least one effect,
// no matter how many ran. This is the counter to compare against Requests.
func (m *Metrics) RecordRequestFaulted() {
	m.requestsFaulted.Add(1)
}

func (m *Metrics) RecordProxied() {
	m.proxied.Add(1)
}

func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		Requests:        m.requests.Load(),
		Matched:         m.matched.Load(),
		FaultsInjected:  m.faultsInjected.Load(),
		RequestsFaulted: m.requestsFaulted.Load(),
		Proxied:         m.proxied.Load(),
	}
}
