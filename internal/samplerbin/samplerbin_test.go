package samplerbin

import "testing"

// elfHeader returns the first bytes of an ELF file with the given byte order and machine type.
func elfHeader(dataEncoding byte, machine uint16) []byte {
	header := make([]byte, 20)
	copy(header, elfMagic)
	header[4] = 2 // 64-bit
	header[elfDataOffset] = dataEncoding
	header[elfMachineOffset] = byte(machine)
	header[elfMachineOffset+1] = byte(machine >> 8)
	return header
}

func TestArchitectureOf(t *testing.T) {
	tests := []struct {
		name       string
		executable []byte
		want       string
	}{
		{name: "x86-64", executable: elfHeader(elfLittleEndian, elfMachineX86_64), want: "amd64"},
		{name: "arm64", executable: elfHeader(elfLittleEndian, elfMachineAArch64), want: "arm64"},
		{name: "another machine", executable: elfHeader(elfLittleEndian, 0x28)},
		{name: "big-endian", executable: elfHeader(2, elfMachineX86_64)},
		{name: "not an ELF file", executable: []byte("MZ\x90\x00 this is a Windows executable")},
		{name: "truncated", executable: elfHeader(elfLittleEndian, elfMachineX86_64)[:10]},
		{name: "empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := architectureOf(tc.executable); got != tc.want {
				t.Errorf("architectureOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// The embedded binary exists only after a build script ran, so this checks consistency rather
// than a fixed answer.
func TestSupportsEngineFollowsEmbeddedBinary(t *testing.T) {
	_, embedded := Binary()
	architecture := Architecture()
	if !embedded {
		if architecture != "" || SupportsEngine("x86_64") || SupportsEngine("aarch64") {
			t.Errorf("no embedded sampler, but Architecture = %q and an engine is supported", architecture)
		}
		return
	}
	engines := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}
	own, known := engines[architecture]
	if !known {
		t.Fatalf("the embedded sampler is not an x86-64 or arm64 ELF executable (Architecture = %q)", architecture)
	}
	for goArch, engine := range engines {
		if got, want := SupportsEngine(engine), engine == own; got != want {
			t.Errorf("SupportsEngine(%q) = %v for a %s sampler, want %v", engine, got, architecture, want)
		}
		if got, want := SupportsEngine(goArch), goArch == architecture; got != want {
			t.Errorf("SupportsEngine(%q) = %v for a %s sampler, want %v", goArch, got, architecture, want)
		}
	}
}
