package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// defaultContextName is the docker CLI's built-in context: $DOCKER_HOST or the platform default.
const defaultContextName = "default"

// EndpointOptions carries the docker CLI global flags that select the engine.
type EndpointOptions struct {
	Host      string // value of the docker global flag --host / -H; empty when not given
	Context   string // value of --context / -c; empty when not given
	ConfigDir string // value of --config; empty means $DOCKER_CONFIG, then ~/.docker
}

// ResolveEndpoint returns the engine endpoint the docker CLI would use for opts, in this order:
// opts.Context, opts.Host, $DOCKER_HOST, $DOCKER_CONTEXT, currentContext of config.json, the
// platform default. It reads only the docker config directory and the environment. Endpoints
// that need TLS or SSH give an error matching ErrUnsupportedEndpoint.
func ResolveEndpoint(opts EndpointOptions) (string, error) {
	if opts.Context != "" {
		return contextEndpoint(opts.ConfigDir, opts.Context)
	}
	if opts.Host != "" {
		return checkHost(opts.Host)
	}
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return checkHost(host)
	}
	if name := os.Getenv("DOCKER_CONTEXT"); name != "" {
		return contextEndpoint(opts.ConfigDir, name)
	}
	configDir, err := resolveConfigDir(opts.ConfigDir)
	if err != nil {
		return "", err
	}
	name, err := currentContext(configDir)
	if err != nil {
		return "", err
	}
	return contextEndpoint(configDir, name)
}

func resolveConfigDir(configDir string) (string, error) {
	if configDir != "" {
		return configDir, nil
	}
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find docker config directory: %w", err)
	}
	return filepath.Join(home, ".docker"), nil
}

func currentContext(configDir string) (string, error) {
	path := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read docker config %s: %w", path, err)
	}
	var config struct {
		CurrentContext string `json:"currentContext"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("parse docker config %s: %w", path, err)
	}
	return config.CurrentContext, nil
}

func contextEndpoint(configDir, name string) (string, error) {
	if name == "" || name == defaultContextName {
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			return checkHost(host)
		}
		return defaultEndpoint, nil
	}
	configDir, err := resolveConfigDir(configDir)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(name))
	hash := hex.EncodeToString(sum[:])

	metaPath := filepath.Join(configDir, "contexts", "meta", hash, "meta.json")
	data, err := os.ReadFile(metaPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("docker context %q not found in %s", name, configDir)
	}
	if err != nil {
		return "", fmt.Errorf("read docker context %q: %w", name, err)
	}
	var meta struct {
		Endpoints struct {
			Docker struct {
				Host string `json:"Host"`
			} `json:"docker"`
		} `json:"Endpoints"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", fmt.Errorf("parse docker context %q: %w", name, err)
	}
	host := meta.Endpoints.Docker.Host
	if host == "" {
		return "", fmt.Errorf("docker context %q has no docker endpoint", name)
	}

	hasTLS, err := nonEmptyDir(filepath.Join(configDir, "contexts", "tls", hash, "docker"))
	if err != nil {
		return "", fmt.Errorf("read tls material of docker context %q: %w", name, err)
	}
	if hasTLS {
		return "", fmt.Errorf("%w: tls endpoint %q of context %q", ErrUnsupportedEndpoint, host, name)
	}
	if err := validateEndpoint(host); err != nil {
		return "", fmt.Errorf("docker context %q: %w", name, err)
	}
	return host, nil
}

func nonEmptyDir(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// checkHost validates a --host or $DOCKER_HOST value. The docker CLI applies
// $DOCKER_TLS_VERIFY to both, which makes a tcp endpoint a TLS one.
func checkHost(host string) (string, error) {
	if err := validateEndpoint(host); err != nil {
		return "", err
	}
	if strings.HasPrefix(host, "tcp://") && os.Getenv("DOCKER_TLS_VERIFY") != "" {
		return "", fmt.Errorf("%w: tls endpoint %q (DOCKER_TLS_VERIFY is set)", ErrUnsupportedEndpoint, host)
	}
	return host, nil
}

// validateEndpoint checks the scheme without platform checks, so resolution behaves the same
// everywhere; New rejects a named pipe outside Windows.
func validateEndpoint(endpoint string) error {
	scheme, rest, ok := strings.Cut(endpoint, "://")
	if !ok {
		return fmt.Errorf("parse docker endpoint %q: missing scheme", endpoint)
	}
	switch scheme {
	case "unix", "npipe", "tcp":
		if strings.Trim(rest, "/") == "" {
			return fmt.Errorf("parse docker endpoint %q: missing address", endpoint)
		}
		return nil
	case "ssh":
		return fmt.Errorf("%w: ssh endpoint %q", ErrUnsupportedEndpoint, endpoint)
	case "https":
		return fmt.Errorf("%w: tls endpoint %q", ErrUnsupportedEndpoint, endpoint)
	default:
		return fmt.Errorf("%w: %s endpoint %q", ErrUnsupportedEndpoint, scheme, endpoint)
	}
}
