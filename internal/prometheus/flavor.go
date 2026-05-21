// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

// Flavor identifies the backend dialect a Client is talking to.
type Flavor int

const (
	// FlavorUnknown means flavor detection has not run yet.
	FlavorUnknown Flavor = iota
	// FlavorProm is upstream Prometheus.
	FlavorProm
	// FlavorVictoria is VictoriaMetrics (single-binary, cluster, or behind vmauth).
	FlavorVictoria
)

// String returns a stable lowercase label.
func (f Flavor) String() string {
	switch f {
	case FlavorProm:
		return "prometheus"
	case FlavorVictoria:
		return "victoria"
	default:
		return "unknown"
	}
}
