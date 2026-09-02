package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var protectedNamespaces = map[string]bool{
	"default":           true,
	"kube-system":       true,
	"kube-public":       true,
	"kube-node-lease":   true,
	"local-path-storage": true,
}

// CleanupPreviousLabResources removes leftover namespaces and objects from prior labs
// so each lab run starts from a clearer cluster state.
func CleanupPreviousLabResources(ctx context.Context, kubeconfigPath string) error {
	if err := deleteLabNamespaces(ctx, kubeconfigPath); err != nil {
		return err
	}
	if err := cleanupDefaultNamespace(ctx, kubeconfigPath); err != nil {
		return err
	}
	_ = deleteLabPersistentVolumes(ctx, kubeconfigPath)
	_ = removeLabTaints(ctx, kubeconfigPath)
	_ = restoreClusterDNSBaseline(ctx, kubeconfigPath)
	return nil
}

func deleteLabNamespaces(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "ns", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("listing namespaces: %w", err)
	}

	var toDelete []string
	for _, ns := range strings.Fields(output) {
		if protectedNamespaces[ns] {
			continue
		}
		toDelete = append(toDelete, ns)
	}
	if len(toDelete) == 0 {
		return nil
	}

	args := append([]string{"delete", "ns", "--wait=false"}, toDelete...)
	if _, err := kubectl(ctx, kubeconfigPath, args...); err != nil {
		return fmt.Errorf("deleting namespaces %v: %w", toDelete, err)
	}

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		remaining := false
		current, err := kubectl(ctx, kubeconfigPath, "get", "ns", "-o", "jsonpath={.items[*].metadata.name}")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		present := map[string]bool{}
		for _, ns := range strings.Fields(current) {
			present[ns] = true
		}
		for _, ns := range toDelete {
			if present[ns] {
				remaining = true
				break
			}
		}
		if !remaining {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for namespaces to delete: %v", toDelete)
}

func cleanupDefaultNamespace(ctx context.Context, kubeconfigPath string) error {
	resources := []string{
		"pods",
		"deployments",
		"replicasets",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"services",
		"ingresses",
		"networkpolicies",
		"persistentvolumeclaims",
		"configmaps",
		"roles",
		"rolebindings",
	}
	for _, res := range resources {
		_, _ = kubectl(ctx, kubeconfigPath, "delete", res, "--all", "-n", "default",
			"--force", "--grace-period=0", "--wait=false", "--ignore-not-found=true")
	}
	return nil
}

func deleteLabPersistentVolumes(ctx context.Context, kubeconfigPath string) error {
	known := []string{"local-pv", "db-pv", "app-pv", "finance-db-pv", "caching-redis-pv", "public-site-pv"}
	for _, name := range known {
		_, _ = kubectl(ctx, kubeconfigPath, "delete", "pv", name, "--ignore-not-found=true", "--wait=false")
	}

	output, err := kubectl(ctx, kubeconfigPath, "get", "pv", "-o", "jsonpath={range .items[*]}{.metadata.name}{' '}{.spec.storageClassName}{'\\n'}{end}")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		sc := ""
		if len(fields) > 1 {
			sc = fields[1]
		}
		// Keep dynamically provisioned volumes tied to system storage; remove manual/exam PVs
		if strings.HasPrefix(name, "pvc-") {
			continue
		}
		if sc == "manual" || sc == "local-storage" || sc == "exam-storage" || sc == "" {
			_, _ = kubectl(ctx, kubeconfigPath, "delete", "pv", name, "--ignore-not-found=true", "--wait=false")
		}
	}
	return nil
}

func removeLabTaints(ctx context.Context, kubeconfigPath string) error {
	nodes, err := kubectl(ctx, kubeconfigPath, "get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return err
	}
	for _, node := range strings.Fields(nodes) {
		_, _ = kubectl(ctx, kubeconfigPath, "taint", "nodes", node, "dedicated=critical:NoSchedule-", "--overwrite")
		_, _ = kubectl(ctx, kubeconfigPath, "label", "nodes", node, "disktype-", "--overwrite")
	}
	return nil
}

func restoreClusterDNSBaseline(ctx context.Context, kubeconfigPath string) error {
	_, _ = kubectl(ctx, kubeconfigPath, "patch", "service", "kube-dns", "-n", "kube-system",
		"--type=json",
		`-p=[{"op":"replace","path":"/spec/selector/k8s-app","value":"kube-dns"}]`)

	replicas, _ := kubectl(ctx, kubeconfigPath, "get", "deployment", "coredns", "-n", "kube-system",
		"-o", "jsonpath={.spec.replicas}")
	if strings.TrimSpace(replicas) == "0" || strings.TrimSpace(replicas) == "" {
		_, _ = kubectl(ctx, kubeconfigPath, "scale", "deployment", "coredns", "-n", "kube-system", "--replicas=2")
	}

	corefile := `apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health {
           lameduck 5s
        }
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
           pods insecure
           fallthrough in-addr.arpa ip6.arpa
           ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf {
           max_concurrent 1000
        }
        cache 30
        loop
        reload
        loadbalance
    }
`
	_ = kubectlApply(ctx, kubeconfigPath, corefile)
	_, _ = kubectl(ctx, kubeconfigPath, "rollout", "restart", "deployment", "coredns", "-n", "kube-system")
	return nil
}
