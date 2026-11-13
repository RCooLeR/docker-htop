// Package docker is a thin Docker Engine API client over raw HTTP. It resolves the engine
// endpoint the way the docker CLI does, talks to unix sockets, Windows named pipes and plain TCP,
// and maps the engine's JSON onto the model types.
//
// The client is read-only except for Kill, SignalProcess and the helper containers and the
// sampler image it creates and removes itself (named docker-htop-* and docker-htop-sampler:*,
// labelled with model.LabelRole).
//
// It does not own TLS or SSH transports, image pulls, builds of user images, or any merging of
// readings; the monitor package does the merging.
package docker
