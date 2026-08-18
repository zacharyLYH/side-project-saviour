package docker

import (
	"context"

	dockerclient "github.com/fsouza/go-dockerclient"
)

// Safe container defaults (the plan's "boring" settings).
const (
	DefaultNetwork = "sps-net"
	DefaultMemory  = 512 << 20 // 512 MiB
)

// Mount is a volume mounted into a container.
type Mount struct {
	Name string // named volume; empty = anonymous (removed with the container)
	Dest string // container path, e.g. /root
}

// Spec describes a container to create or run. Zero values get safe
// defaults: non-privileged, read-only rootfs, 512 MiB memory cap, sps-net
// network.
type Spec struct {
	Name     string
	Image    string
	Cmd      []string
	Env      []string
	WorkDir  string
	Writable bool   // opt out of the read-only rootfs
	Memory   int64  // 0 = DefaultMemory
	Network  string // "" = DefaultNetwork
	Volumes  []Mount
	Binds    []string // host path:container path[:ro] (e.g. an SSH key)
}

// containerOptions translates a Spec into go-dockerclient options. Pure, so
// it is unit-tested without a Docker engine.
func containerOptions(ctx context.Context, spec Spec) dockerclient.CreateContainerOptions {
	network := spec.Network
	if network == "" {
		network = DefaultNetwork
	}
	mem := spec.Memory
	if mem == 0 {
		mem = DefaultMemory
	}
	host := &dockerclient.HostConfig{
		NetworkMode:    network,
		ReadonlyRootfs: !spec.Writable,
		Memory:         mem,
		Binds:          spec.Binds,
	}
	for _, m := range spec.Volumes {
		host.Mounts = append(host.Mounts, dockerclient.HostMount{Type: "volume", Source: m.Name, Target: m.Dest})
	}
	return dockerclient.CreateContainerOptions{
		Name:       spec.Name,
		Config:     &dockerclient.Config{Image: spec.Image, Cmd: spec.Cmd, Env: spec.Env, WorkingDir: spec.WorkDir},
		HostConfig: host,
		Context:    ctx,
	}
}
