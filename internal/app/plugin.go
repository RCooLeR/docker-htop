package app

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/RCooLeR/docker-htop/internal/buildinfo"
)

// MetadataCommand is the argument the docker CLI passes to a plugin binary to ask what it is.
// A binary in cli-plugins that does not answer it is not listed as a plugin.
const MetadataCommand = "docker-cli-plugin-metadata"

// Description is the one-line summary shown by `docker --help` and by the plugin's own help.
const Description = "Live container and process monitor (docker stats + htop)"

// metadata is the answer format of the docker CLI plugin protocol, schema 0.1.0.
type metadata struct {
	SchemaVersion    string `json:"SchemaVersion"`
	Vendor           string `json:"Vendor"`
	Version          string `json:"Version"`
	ShortDescription string `json:"ShortDescription"`
}

// WriteMetadata answers the docker CLI's plugin metadata request on w.
func WriteMetadata(w io.Writer) error {
	answer := metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "RCooLeR",
		Version:          buildinfo.Version,
		ShortDescription: Description,
	}
	if err := json.NewEncoder(w).Encode(answer); err != nil {
		return fmt.Errorf("write plugin metadata: %w", err)
	}
	return nil
}
