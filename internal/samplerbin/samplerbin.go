// Package samplerbin embeds the Linux sampler binary, so the plugin can build its helper image
// on any engine without a registry. The build scripts compile cmd/docker-htop-sampler into bin/
// for the plugin's own architecture before they build the plugin; a plain `go build` finds only
// the placeholder file and produces a plugin that never uses helper containers.
package samplerbin

import (
	"embed"
	"encoding/binary"
	"slices"
)

//go:embed all:bin
var files embed.FS

const binaryPath = "bin/docker-htop-sampler"

// ELF header facts used to tell which machine the embedded binary was built for.
const (
	elfMagic          = "\x7fELF"
	elfDataOffset     = 5 // EI_DATA: 1 is little-endian
	elfLittleEndian   = 1
	elfMachineOffset  = 18 // e_machine, two bytes
	elfMachineX86_64  = 0x3E
	elfMachineAArch64 = 0xB7
)

// Architecture names in Go's and the OCI image format's spelling.
const (
	archAMD64 = "amd64"
	archARM64 = "arm64"
)

// Binary returns the embedded sampler executable. ok is false when the plugin was built
// without it.
func Binary() (data []byte, ok bool) {
	data, err := files.ReadFile(binaryPath)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// Architecture returns the architecture the embedded sampler was built for, in the spelling of
// Go and of the OCI image format ("amd64", "arm64"). It is "" when there is no sampler or its
// machine type is not one the plugin knows.
func Architecture() string {
	data, ok := Binary()
	if !ok {
		return ""
	}
	return architectureOf(data)
}

// SupportsEngine reports whether the embedded sampler can run on an engine with the given
// architecture, written the way the Engine API reports it ("x86_64", "aarch64").
func SupportsEngine(engineArchitecture string) bool {
	switch Architecture() {
	case archAMD64:
		return slices.Contains([]string{"x86_64", archAMD64}, engineArchitecture)
	case archARM64:
		return slices.Contains([]string{"aarch64", archARM64}, engineArchitecture)
	default:
		return false
	}
}

// architectureOf reads the machine type from the header of a little-endian ELF executable.
func architectureOf(executable []byte) string {
	if len(executable) < elfMachineOffset+2 || string(executable[:len(elfMagic)]) != elfMagic ||
		executable[elfDataOffset] != elfLittleEndian {
		return ""
	}
	switch binary.LittleEndian.Uint16(executable[elfMachineOffset:]) {
	case elfMachineX86_64:
		return archAMD64
	case elfMachineAArch64:
		return archARM64
	default:
		return ""
	}
}
