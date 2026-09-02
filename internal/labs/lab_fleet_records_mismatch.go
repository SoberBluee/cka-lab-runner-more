package labs

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(&FleetRecordsMismatchLab{})
}

type FleetRecordsMismatchLab struct {
	BaseLab
}

func (l *FleetRecordsMismatchLab) ID() string {
	return "fleet_records_mismatch"
}

func (l *FleetRecordsMismatchLab) Title() string {
	return "Fleet Records Disagree With The Cluster"
}

func (l *FleetRecordsMismatchLab) Category() Category {
	return CategoryControlPlane
}

func (l *FleetRecordsMismatchLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *FleetRecordsMismatchLab) Description() string {
	return `An audit flagged the inventory record for this cluster. The ConfigMap
'fleet-records' in kube-system claims versions that nobody has confirmed against the
live cluster, and the change board will not approve the next maintenance window until
the record is accurate.

Your task: correct every value in the 'fleet-records' ConfigMap so it matches what the
cluster actually reports, and flip 'verified' to "true".

Keys to fill in: node-name, kubelet-version, apiserver-version, runtime-version.
Use the version string exactly as the cluster reports it (for example v1.30.0).`
}

func (l *FleetRecordsMismatchLab) Hints() []string {
	return []string{
		"kubectl get nodes -o wide reports more than you think — kubelet version and container runtime included",
		"kubectl get node <name> -o yaml has a nodeInfo block with the authoritative values",
		"The API server runs as a static pod; its image tag is the version it is actually running",
		"kubectl -n kube-system get pod -l component=kube-apiserver -o jsonpath='{.items[0].spec.containers[0].image}'",
		"Patch the ConfigMap with kubectl patch cm fleet-records -n kube-system --type merge -p '{\"data\":{...}}'",
	}
}

func (l *FleetRecordsMismatchLab) EstimatedTime() int {
	return 20
}

func (l *FleetRecordsMismatchLab) Tags() []string {
	return []string{"control-plane", "nodes", "static-pods", "procedure"}
}

func (l *FleetRecordsMismatchLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *FleetRecordsMismatchLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: fleet-records
  namespace: kube-system
data:
  node-name: "node-unknown"
  kubelet-version: "v1.27.4"
  apiserver-version: "v1.28.1"
  runtime-version: "docker://20.10.7"
  verified: "false"
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying fleet records: %w", err)
	}
	return nil
}

func (l *FleetRecordsMismatchLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	_, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "fleet-records", "-n", "kube-system")
	if err != nil {
		return fmt.Errorf("fleet-records ConfigMap not found")
	}
	return nil
}

func (l *FleetRecordsMismatchLab) Verify(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	nodeInfo, err := kubectl(ctx, kubeconfigPath, "get", "node", node,
		"-o", "jsonpath={.status.nodeInfo.kubeletVersion}{'|'}{.status.nodeInfo.containerRuntimeVersion}")
	if err != nil {
		return fmt.Errorf("reading node info: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(nodeInfo), "|")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected node info output: %q", nodeInfo)
	}
	wantKubelet, wantRuntime := parts[0], parts[1]

	apiserverImage, err := staticPodImage(ctx, kubeconfigPath, "kube-apiserver")
	if err != nil {
		return err
	}
	wantAPIServer := imageTag(apiserverImage)
	if wantAPIServer == "" {
		return fmt.Errorf("could not read API server image tag from %q", apiserverImage)
	}

	checks := []struct {
		key  string
		want string
	}{
		{"node-name", node},
		{"kubelet-version", wantKubelet},
		{"apiserver-version", wantAPIServer},
		{"runtime-version", wantRuntime},
		{"verified", "true"},
	}

	for _, check := range checks {
		got, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "fleet-records", "-n", "kube-system",
			"-o", fmt.Sprintf("jsonpath={.data.%s}", check.key))
		if err != nil {
			return fmt.Errorf("reading fleet-records: %w", err)
		}
		got = strings.TrimSpace(got)
		if !versionEqual(got, check.want) {
			return fmt.Errorf("fleet-records %s is %q, cluster reports %q", check.key, got, check.want)
		}
	}

	return nil
}

// versionEqual compares record values, tolerating a missing or extra "v" prefix
// on version strings.
func versionEqual(got, want string) bool {
	return strings.TrimPrefix(got, "v") == strings.TrimPrefix(want, "v")
}

func (l *FleetRecordsMismatchLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Read the record that needs correcting",
			Command:     "kubectl get configmap fleet-records -n kube-system -o yaml",
		},
		{
			Description: "Get the node name, kubelet version and container runtime",
			Command:     "kubectl get nodes -o wide",
			Notes:       "Exact strings live in kubectl get node <name> -o jsonpath='{.status.nodeInfo}'",
		},
		{
			Description: "Get the version the API server is really running",
			Command:     "kubectl -n kube-system get pod -l component=kube-apiserver -o jsonpath='{.items[0].spec.containers[0].image}'",
			Notes:       "Same value appears in /etc/kubernetes/manifests/kube-apiserver.yaml on the control plane node",
		},
		{
			Description: "Correct the record",
			Command: `kubectl patch configmap fleet-records -n kube-system --type merge -p '{"data":{
  "node-name":"<node>",
  "kubelet-version":"<kubelet>",
  "apiserver-version":"<apiserver>",
  "runtime-version":"<runtime>",
  "verified":"true"}}'`,
		},
		{
			Description: "Confirm the record matches the cluster",
			Command:     "kubectl get configmap fleet-records -n kube-system -o jsonpath='{.data}' && kubectl get nodes -o wide",
		},
	}
}
