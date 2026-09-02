package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&DNSUnreachableLab{})
}

type DNSUnreachableLab struct {
	BaseLab
}

func (l *DNSUnreachableLab) ID() string {
	return "dns_unreachable"
}

func (l *DNSUnreachableLab) Title() string {
	return "Cluster DNS Unreachable"
}

func (l *DNSUnreachableLab) Category() Category {
	return CategoryDNS
}

func (l *DNSUnreachableLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *DNSUnreachableLab) Description() string {
	return `Pods cannot resolve Service names. DNS pods in kube-system appear Running,
but clients still fail when querying cluster DNS.

Your task: Fix cluster DNS so Service name resolution works again.`
}

func (l *DNSUnreachableLab) Hints() []string {
	return []string{
		"DNS pods Running does not guarantee the DNS Service has backends",
		"Inspect the kube-dns / CoreDNS Service selectors and Endpoints",
		"Compare Service selector labels with the DNS pod labels",
		"Pods use the cluster DNS Service ClusterIP from /etc/resolv.conf",
	}
}

func (l *DNSUnreachableLab) EstimatedTime() int {
	return 20
}

func (l *DNSUnreachableLab) Tags() []string {
	return []string{"dns", "services", "endpoints", "troubleshooting"}
}

func (l *DNSUnreachableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *DNSUnreachableLab) Break(ctx context.Context, kubeconfigPath string) error {
	// Wrong selector → DNS pods stay Running but kube-dns Service has no Endpoints
	_, err := kubectl(ctx, kubeconfigPath, "patch", "service", "kube-dns", "-n", "kube-system",
		"--type=json",
		`-p=[{"op":"replace","path":"/spec/selector/k8s-app","value":"kube-dns-broken"}]`)
	if err != nil {
		return fmt.Errorf("patching kube-dns service: %w", err)
	}
	return nil
}

func (l *DNSUnreachableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)
	eps, _ := kubectl(ctx, kubeconfigPath, "get", "endpoints", "kube-dns", "-n", "kube-system",
		"-o", "jsonpath={.subsets[*].addresses[*].ip}")
	if strings.TrimSpace(eps) == "" {
		return nil
	}
	return fmt.Errorf("expected kube-dns endpoints empty, got %q", eps)
}

func (l *DNSUnreachableLab) Verify(ctx context.Context, kubeconfigPath string) error {
	eps, err := kubectl(ctx, kubeconfigPath, "get", "endpoints", "kube-dns", "-n", "kube-system",
		"-o", "jsonpath={.subsets[*].addresses[*].ip}")
	if err != nil {
		return fmt.Errorf("failed to check kube-dns endpoints: %w", err)
	}
	if strings.TrimSpace(eps) == "" {
		return fmt.Errorf("kube-dns service still has no endpoints")
	}

	selector, err := kubectl(ctx, kubeconfigPath, "get", "svc", "kube-dns", "-n", "kube-system",
		"-o", "jsonpath={.spec.selector.k8s-app}")
	if err != nil {
		return fmt.Errorf("failed to check kube-dns selector: %w", err)
	}
	if strings.TrimSpace(selector) != "kube-dns" {
		return fmt.Errorf("kube-dns selector still wrong (k8s-app=%s)", selector)
	}

	_, err = kubectl(ctx, kubeconfigPath, "run", "dns-verify-unreach", "--image=busybox:1.28",
		"--rm", "-i", "--restart=Never", "--", "nslookup", "kubernetes.default.svc.cluster.local")
	if err != nil {
		return fmt.Errorf("DNS lookup still failing: %w", err)
	}
	return nil
}

func (l *DNSUnreachableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm DNS pods are Running",
			Command:     "kubectl get pods -n kube-system -l k8s-app=kube-dns",
		},
		{
			Description: "Check the DNS Service and Endpoints",
			Command:     "kubectl get svc,endpoints kube-dns -n kube-system",
			Notes:       "Endpoints will be empty if the selector is wrong",
		},
		{
			Description: "Compare labels",
			Command:     "kubectl get pods -n kube-system -l k8s-app=kube-dns --show-labels; kubectl get svc kube-dns -n kube-system -o yaml | grep -A3 selector",
			Notes:       "Service selects k8s-app=kube-dns-broken but pods have k8s-app=kube-dns",
		},
		{
			Description: "Fix the Service selector",
			Command:     "kubectl patch svc kube-dns -n kube-system --type=json -p='[{\"op\":\"replace\",\"path\":\"/spec/selector/k8s-app\",\"value\":\"kube-dns\"}]'",
		},
		{
			Description: "Verify Endpoints and DNS",
			Command:     "kubectl get endpoints kube-dns -n kube-system; kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
