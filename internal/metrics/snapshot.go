package metrics

type Snapshot struct {
	Requests       uint64 `json:"requests"`
	Matched        uint64 `json:"matched"`
	FaultsInjected uint64 `json:"faultsInjected"`
	Proxied        uint64 `json:"proxied"`
}
