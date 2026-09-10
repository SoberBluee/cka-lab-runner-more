package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var protectedNamespaces = map[string]bool{
	"default":            true,
	"kube-system":        true,
	"kube-public":        true,
	"kube-node-lease":    true,
	"local-path-storage": true,
}

// CleanupPreviousLabResources removes leftover namespaces and objects from prior labs
// so each lab run starts from a clearer cluster state.
func CleanupPreviousLabResources(ctx context.Context, kubeconfigPath string) error {
	_ = cleanupMockExamResources(ctx, kubeconfigPath)
	_ = cleanupMockExam02Resources(ctx, kubeconfigPath)
	if err := deleteLabNamespaces(ctx, kubeconfigPath); err != nil {
		return err
	}
	if err := cleanupDefaultNamespace(ctx, kubeconfigPath); err != nil {
		return err
	}
	_ = deleteLabPersistentVolumes(ctx, kubeconfigPath)
	_ = deleteLabCRDs(ctx, kubeconfigPath)
	_ = deleteLabClusterRoleBindings(ctx, kubeconfigPath)
	_ = removeLabTaints(ctx, kubeconfigPath)
	_ = uncordonNodes(ctx, kubeconfigPath)
	_ = restoreClusterDNSBaseline(ctx, kubeconfigPath)
	_ = restoreKubeProxyBaseline(ctx, kubeconfigPath)
	_ = restoreControlPlaneImages(ctx, kubeconfigPath)
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
		"horizontalpodautoscalers",
		"verticalpodautoscalers",
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
	known := []string{"local-pv", "db-pv", "app-pv", "finance-db-pv", "caching-redis-pv", "public-site-pv", "audit-logs-pv"}
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

// labTaintKeys are taint keys applied by labs; cleanup strips them so a stale
// taint from an abandoned lab cannot break the next one.
var labTaintKeys = []string{
	"dedicated",
	"tenancy",
	"tier",
	"hardened",
	"maintenance",
}

func removeLabTaints(ctx context.Context, kubeconfigPath string) error {
	nodes, err := kubectl(ctx, kubeconfigPath, "get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return err
	}
	for _, node := range strings.Fields(nodes) {
		for _, key := range labTaintKeys {
			_, _ = kubectl(ctx, kubeconfigPath, "taint", "nodes", node, key+"-")
		}
		_, _ = kubectl(ctx, kubeconfigPath, "label", "nodes", node,
			"disktype-", "node-type-", "node-pool-", "--overwrite")
	}
	return nil
}

func uncordonNodes(ctx context.Context, kubeconfigPath string) error {
	nodes, err := kubectl(ctx, kubeconfigPath, "get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return err
	}
	for _, node := range strings.Fields(nodes) {
		_, _ = kubectl(ctx, kubeconfigPath, "uncordon", node)
	}
	return nil
}

// labCRDNames are CustomResourceDefinitions installed by labs. They are cluster
// scoped, so deleting lab namespaces is not enough to remove them.
var labCRDNames = []string{
	"retentionpolicies.ops.cka.local",
	"backuppolicies.ops.cka.local",
	"verticalpodautoscalers.autoscaling.k8s.io",
	"verticalpodautoscalercheckpoints.autoscaling.k8s.io",
	"gatewayclasses.gateway.networking.k8s.io",
	"gateways.gateway.networking.k8s.io",
	"httproutes.gateway.networking.k8s.io",
	"widgets.ops.exam.local",
}

func deleteLabCRDs(ctx context.Context, kubeconfigPath string) error {
	for _, name := range labCRDNames {
		_, _ = kubectl(ctx, kubeconfigPath, "delete", "crd", name, "--ignore-not-found=true", "--wait=false")
	}
	return nil
}

// deleteLabClusterRoleBindings removes cluster-wide bindings that grant access
// to ServiceAccounts living in lab namespaces. Those bindings survive namespace
// deletion and would otherwise hand a later lab permissions it should not have.
func deleteLabClusterRoleBindings(ctx context.Context, kubeconfigPath string) error {
	labSubjectNamespaces := map[string]bool{
		"reporting":  true,
		"warehouse":  true,
		"edge":       true,
		"ci":         true,
		"apps":       true,
		"ops":        true,
		"commerce":   true,
		"storefront": true,
		"portal":     true,
		"billing":    true,
	}

	output, err := kubectl(ctx, kubeconfigPath, "get", "clusterrolebindings",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{'|'}{range .subjects[*]}{.namespace}{','}{end}{'\\n'}{end}")
	if err != nil {
		return err
	}

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		for _, ns := range strings.Split(parts[1], ",") {
			if labSubjectNamespaces[strings.TrimSpace(ns)] {
				_, _ = kubectl(ctx, kubeconfigPath, "delete", "clusterrolebinding", parts[0],
					"--ignore-not-found=true", "--wait=false")
				break
			}
		}
	}
	return nil
}

func restoreKubeProxyBaseline(ctx context.Context, kubeconfigPath string) error {
	selector, _ := kubectl(ctx, kubeconfigPath, "get", "daemonset", "kube-proxy", "-n", "kube-system",
		"-o", "jsonpath={.spec.template.spec.nodeSelector}")
	if strings.Contains(selector, labProxySelectorKey) {
		_, _ = kubectl(ctx, kubeconfigPath, "patch", "daemonset", "kube-proxy", "-n", "kube-system",
			"--type=json", `-p=[{"op":"remove","path":"/spec/template/spec/nodeSelector/cka-lab~1proxy"}]`)
	}
	return nil
}

// restoreControlPlaneImages realigns control plane static pod images with the
// API server image tag. A lab that pins a component to a mismatched tag leaves
// the cluster unusable if the user walks away mid-exercise.
func restoreControlPlaneImages(ctx context.Context, kubeconfigPath string) error {
	apiserverImage, err := staticPodImage(ctx, kubeconfigPath, "kube-apiserver")
	if err != nil {
		return err
	}
	tag := imageTag(apiserverImage)
	if tag == "" {
		return fmt.Errorf("could not determine API server image tag")
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	for _, component := range []string{"kube-controller-manager", "kube-scheduler"} {
		manifest := fmt.Sprintf("/etc/kubernetes/manifests/%s.yaml", component)
		script := fmt.Sprintf(
			"grep -q 'image: registry.k8s.io/%s:%s' %s || sed -i 's|image: registry.k8s.io/%s:.*|image: registry.k8s.io/%s:%s|' %s",
			component, tag, manifest, component, component, tag, manifest)
		_, _ = dockerExec(ctx, node, "sh", "-c", script)
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

	volume, _ := kubectl(ctx, kubeconfigPath, "get", "deployment", "coredns", "-n", "kube-system",
		"-o", "jsonpath={.spec.template.spec.volumes[0].name}")
	volumeName := strings.TrimSpace(volume)
	if volumeName == "" {
		volumeName = "config-volume"
	}
	_, _ = kubectl(ctx, kubeconfigPath, "patch", "deployment", "coredns", "-n", "kube-system",
		"-p", fmt.Sprintf(`{"spec":{"template":{"spec":{"volumes":[{"name":%q,"configMap":{"name":"coredns","items":[{"key":"Corefile","path":"Corefile"}]}}]}}}}`, volumeName))

	clusterRole := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "system:coredns"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
rules:
- apiGroups: [""]
  resources: ["endpoints", "services", "pods", "namespaces"]
  verbs: ["list", "watch"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["list", "watch"]
`
	_ = kubectlApply(ctx, kubeconfigPath, clusterRole)

	_, _ = kubectl(ctx, kubeconfigPath, "rollout", "restart", "deployment", "coredns", "-n", "kube-system")
	return nil
}
