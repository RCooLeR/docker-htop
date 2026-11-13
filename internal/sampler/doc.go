// Package sampler reads a Linux procfs tree and turns it into model.Sample readings: per-core CPU
// counters, memory, load, task counts and the processes that belong to Docker containers. It also
// streams those readings as JSON lines and sends a signal to one container process after checking
// its cgroup.
//
// It runs inside the helper container that shares the Docker host's PID namespace. It does not
// talk to the Docker engine, resolve user names or compute rates: the plugin does that from two
// samples.
//
// All parsing takes the proc root as a parameter and uses ordinary file reads, so it can be tested
// on any operating system against a fixture tree. Only sending a signal is Linux specific.
package sampler
