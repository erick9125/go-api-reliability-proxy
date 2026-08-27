package metrics

// Snapshot is a point-in-time read of the counters, described in the README
// under Metrics.
type Snapshot struct {
	// Requests excludes the reserved /__reliability/* endpoints.
	Requests uint64 `json:"requests"`
	Matched  uint64 `json:"matched"`
	// FaultsInjected counts effects; one request can contribute more than one.
	FaultsInjected uint64 `json:"faultsInjected"`
	// RequestsFaulted counts requests that experienced at least one effect.
	RequestsFaulted uint64 `json:"requestsFaulted"`
	// Proxied counts requests forwarded upstream, whatever the upstream replied.
	Proxied uint64 `json:"proxied"`
}
