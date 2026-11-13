// Package ui owns the terminal presentation of DockerHtop: the full-screen Bubble Tea program
// (header meters, the project / container / process tree, prompts, dialogs and the key bar) and
// the plain one-shot renderer used by --no-stream.
//
// It does not collect data or talk to Docker. Snapshots arrive on a channel from the monitor, and
// every side effect (signals, logs, shell) goes through the Actions interface, called only after
// an explicit user confirmation.
package ui
