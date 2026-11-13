// Package buildinfo carries the version stamped into the binaries at build time.
package buildinfo

// Version is set by scripts/build.ps1 through
// -ldflags "-X github.com/RCooLeR/docker-htop/internal/buildinfo.Version=<version>".
var Version = "dev"
