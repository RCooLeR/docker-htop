// Package monitor turns Engine API readings and sampler samples into one model.Snapshot per
// polling interval. It owns the polling loop, rate derivation from cumulative counters, the
// switch between model.ModeSampler and model.ModeAPI, and the per-container user-name cache.
//
// It does not talk to Docker itself (an Engine does), does not read /proc (the sampler does)
// and does not draw anything (the UI consumes the snapshots).
package monitor
