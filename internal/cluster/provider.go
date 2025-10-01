package cluster

import (
	"context"
	"fmt"
)

// Provider defines the interface for cluster providers (kind, k3d, etc.)
type Provider interface {
	// Up creates or ensures the cluster exists
	Up(ctx context.Context) error

	// Down deletes the cluster
	Down(ctx context.Context) error

	// Exists checks if the cluster already exists
	Exists(ctx context.Context) (bool, error)

	// KubeconfigPath returns the path to the kubeconfig file
	KubeconfigPath(ctx context.Context) (string, error)

	// Name returns the cluster name
	Name() string
}

// Config holds configuration for creating a cluster provider
type Config struct {
	Provider          string
	Name              string
	KubernetesVersion string
}

// NewProvider creates a new cluster provider based on the config
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "kind":
		return NewKindProvider(cfg.Name, cfg.KubernetesVersion), nil
	case "k3d":
		return nil, fmt.Errorf("k3d provider not yet implemented")
	case "minikube":
		return nil, fmt.Errorf("minikube provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: kind, k3d, minikube)", cfg.Provider)
	}
}
