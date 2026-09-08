package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&DNSPodsNeverReadyLab{})
}

type DNSPodsNeverReadyLab struct {
	BaseLab
}

func (l *DNSPodsNeverReadyLab) ID() string {
	return "dns_pods_never_ready"
}

func (l *DNSPodsNeverReadyLab) Title() string {
	return "DNS Pods Never Ready"
}

func (l *DNSPodsNeverReadyLab) Category() Category {
	return CategoryDNS
}

func (l *DNSPodsNeverReadyLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *DNSPodsNeverReadyLab) Description() string {
	return `Name resolution has stopped working for every pod in the cluster. The DNS pods in
kube-system are Running and are not restarting, but they never report Ready, so the
kube-dns Service has no endpoints. The Corefile has not been modified.

Your task: Get the DNS pods Ready and restore name resolution.`
}

func (l *DNSPodsNeverReadyLab) Hints() []string {
	return []string{
		"kubectl -n kube-system get pods -l k8s-app=kube-dns shows Running with 0 restarts, so the process itself is alive",
		"kubectl -n kube-system get endpoints kube-dns is empty — unready pods are never added to a Service",
		"kubectl -n kube-system describe pod shows which probe is failing",
		"kubectl logs -n kube-system -l k8s-app=kube-dns says what CoreDNS is stuck waiting for",
		"CoreDNS only reports ready once it has read the API; check what its ServiceAccount is allowed to do",
	}
}

func (l *DNSPodsNeverReadyLab) EstimatedTime() int {
	return 30
}

func (l *DNSPodsNeverReadyLab) Tags() []string {
	return []string{"dns", "coredns", "rbac", "troubleshooting"}
}

func (l *DNSPodsNeverReadyLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return waitDNSPodsReady(ctx, kubeconfigPath)
}

func (l *DNSPodsNeverReadyLab) Break(ctx context.Context, kubeconfigPath string) error {
	patch := `[{"op":"replace","path":"/rules","value":[{"apiGroups":[""],"resources":["namespaces"],"verbs":["list","watch"]}]}]`
	if output, err := kubectl(ctx, kubeconfigPath, "patch", "clusterrole", "system:coredns",
		"--type=json", "-p="+patch); err != nil {
		return fmt.Errorf("stripping CoreDNS permissions: %s: %w", output, err)
	}

	if output, err := kubectl(ctx, kubeconfigPath, "rollout", "restart", "deployment", "coredns",
		"-n", "kube-system"); err != nil {
		return fmt.Errorf("restarting CoreDNS: %s: %w", output, err)
	}
	// The new pods can never pass their readiness probe, so settling is the
	// absence of readiness rather than waitDNSPodsReady.
	_ = waitDNSPodsUnready(ctx, kubeconfigPath)
	return nil
}

func (l *DNSPodsNeverReadyLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	return waitDNSPodsUnready(ctx, kubeconfigPath)
}

func (l *DNSPodsNeverReadyLab) Verify(ctx context.Context, kubeconfigPath string) error {
	for _, resource := range []string{"services", "endpointslices.discovery.k8s.io"} {
		if !coreDNSCanList(ctx, kubeconfigPath, resource) {
			return fmt.Errorf("the coredns ServiceAccount still cannot list %s", resource)
		}
	}

	if err := waitDNSPodsReady(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("DNS pods are still not Ready (if they are stuck in watch backoff, run: kubectl -n kube-system rollout restart deployment coredns): %w", err)
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := dnsLookupFromTempPod(ctx, kubeconfigPath, "dns-verify-ready",
			"kubernetes.default.svc.cluster.local")
		if err != nil {
			return fmt.Errorf("Service DNS lookup still failing: %w", err)
		}
		if !dnsLookupAnswered(output) {
			return fmt.Errorf("lookup returned no answer: %s", output)
		}
		return nil
	})
}

func coreDNSCanList(ctx context.Context, kubeconfigPath, resource string) bool {
	output, err := kubectl(ctx, kubeconfigPath, "auth", "can-i", "list", resource,
		"--as=system:serviceaccount:kube-system:coredns")
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(output), "yes")
}

// waitDNSPodsUnready waits until no CoreDNS pod reports Ready.
func waitDNSPodsUnready(ctx context.Context, kubeconfigPath string) error {
	return waitFor(ctx, 90*time.Second, func() error {
		ready, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
			"-l", "k8s-app=kube-dns",
			"-o", "jsonpath={.items[*].status.containerStatuses[*].ready}")
		if err != nil {
			return err
		}
		if strings.Contains(ready, "true") {
			return fmt.Errorf("DNS pods are still Ready")
		}
		return nil
	})
}

func (l *DNSPodsNeverReadyLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Look at the DNS pods and their Service endpoints",
			Command:     "kubectl -n kube-system get pods -l k8s-app=kube-dns && kubectl -n kube-system get endpoints kube-dns",
			Notes:       "Running, 0 restarts, 0/1 Ready — and no endpoints, which is why every lookup fails",
		},
		{
			Description: "Find the failing probe",
			Command:     "kubectl -n kube-system describe pod -l k8s-app=kube-dns | tail -20",
			Notes:       "Readiness probe failed on the /ready endpoint",
		},
		{
			Description: "Read the CoreDNS logs",
			Command:     "kubectl logs -n kube-system -l k8s-app=kube-dns --tail=20",
			Notes:       "Failed to list *v1.Service: services is forbidden — the kubernetes plugin cannot sync, so CoreDNS never reports ready",
		},
		{
			Description: "Confirm the missing permission",
			Command:     "kubectl auth can-i list services --as=system:serviceaccount:kube-system:coredns",
			Notes:       "Returns no; the ClusterRoleBinding system:coredns binds that ServiceAccount to the ClusterRole system:coredns",
		},
		{
			Description: "Inspect the damaged ClusterRole",
			Command:     "kubectl get clusterrole system:coredns -o yaml",
			Notes:       "Only the namespaces rule is left",
		},
		{
			Description: "Restore the default CoreDNS rules",
			Command:     "kubectl edit clusterrole system:coredns",
			Notes: `rules:
- apiGroups: [""]
  resources: ["endpoints", "services", "pods", "namespaces"]
  verbs: ["list", "watch"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["list", "watch"]`,
		},
		{
			Description: "Restart CoreDNS so the watches start immediately",
			Command:     "kubectl -n kube-system rollout restart deployment coredns",
		},
		{
			Description: "Retest resolution",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
