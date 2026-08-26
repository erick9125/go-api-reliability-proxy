package metrics

// Snapshot is a point-in-time read of the counters. Each field is described in
// the README under Metrics; the distinction that matters is FaultsInjected
// counting effects while RequestsFaulted counts requests.
type Snapshot struct {
	// Requests counts requests that reached the proxy, excluding the reserved
	// /__reliability/* endpoints.
	Requests uint64 `json:"requests"`
	// Matched counts requests that matched a rule.
	Matched uint64 `json:"matched"`
	// FaultsInjected counts individual effects that took hold. One request can
	// contribute more than one.
	FaultsInjected uint64 `json:"faultsInjected"`
	// RequestsFaulted counts requests that experienced at least one effect.
	RequestsFaulted uint64 `json:"requestsFaulted"`
	// Proxied counts requests forwarded upstream, regardless of how the
	// upstream then responded.
	Proxied uint64 `json:"proxied"`
}
