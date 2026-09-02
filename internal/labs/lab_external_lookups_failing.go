package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&ExternalLookupsFailingLab{})
}

type ExternalLookupsFailingLab struct {
	BaseLab
}

func (l *ExternalLookupsFailingLab) ID() string {
	return "external_lookups_failing"
}

func (l *ExternalLookupsFailingLab) Title() string {
	return "External Lookups Failing"
}

func (l *ExternalLookupsFailingLab) Category() Category {
	return CategoryDNS
}

func (l *ExternalLookupsFailingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *ExternalLookupsFailingLab) Description() string {
	return `Pods can resolve in-cluster Service names, but cannot resolve external
hostnames (for example kubernetes.google.com). Outbound internet from nodes may still work.

Your task: Restore external DNS resolution for pods.`
}

func (l *ExternalLookupsFailingLab) Hints() []string {
	return []string{
		"Compare nslookup for kubernetes.default.svc.cluster.local vs an external name",
		"Cluster DNS often forwards unmatched queries to an upstream resolver",
		"Inspect the DNS server configuration for the forward / upstream settings",
		"Restart DNS pods after correcting the configuration",
	}
}

func (l *ExternalLookupsFailingLab) EstimatedTime() int {
	return 20
}

func (l *ExternalLookupsFailingLab) Tags() []string {
	return []string{"dns", "forward", "troubleshooting"}
}

func (l *ExternalLookupsFailingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ExternalLookupsFailingLab) Break(ctx context.Context, kubeconfigPath string) error {
	broken := `apiVersion: v1
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
        forward . 192.0.2.1 {
           max_concurrent 1000
        }
        cache 30
        loop
        reload
        loadbalance
    }
`
	if err := kubectlApply(ctx, kubeconfigPath, broken); err != nil {
		return fmt.Errorf("applying broken forward config: %w", err)
	}
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pods", "-n", "kube-system", "-l", "k8s-app=kube-dns", "--force", "--grace-period=0")
	return nil
}

func (l *ExternalLookupsFailingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(15 * time.Second)
	return nil
}

func (l *ExternalLookupsFailingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if err := waitDNSPodsReady(ctx, kubeconfigPath); err != nil {
		return err
	}

	output, err := kubectl(ctx, kubeconfigPath, "run", "dns-verify-ext", "--image=busybox:1.28",
		"--rm", "-i", "--restart=Never", "--", "nslookup", "kubernetes.default.svc.cluster.local")
	if err != nil {
		return fmt.Errorf("cluster DNS still broken: %w", err)
	}
	if !strings.Contains(strings.ToLower(output), "address") && !strings.Contains(output, "Name:") {
		return fmt.Errorf("cluster DNS unexpected output: %s", output)
	}

	output, err = kubectl(ctx, kubeconfigPath, "run", "dns-verify-ext2", "--image=busybox:1.28",
		"--rm", "-i", "--restart=Never", "--", "nslookup", "kubernetes.io")
	if err != nil {
		return fmt.Errorf("external DNS lookup failed: %w", err)
	}
	lower := strings.ToLower(output)
	if strings.Contains(lower, "can't resolve") || strings.Contains(lower, "no servers could be reached") {
		return fmt.Errorf("external DNS still failing: %s", output)
	}
	if !strings.Contains(lower, "address") && !strings.Contains(output, "Name:") {
		return fmt.Errorf("external DNS unexpected output: %s", output)
	}
	return nil
}

func (l *ExternalLookupsFailingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Compare cluster vs external lookups",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local; kubectl run tmp2 --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.google.com",
		},
		{
			Description: "Inspect DNS configuration",
			Command:     "kubectl get configmap coredns -n kube-system -o yaml | grep -A5 forward",
			Notes:       "forward points at 192.0.2.1 (documentation/test address) instead of /etc/resolv.conf",
		},
		{
			Description: "Restore a working upstream",
			Command:     "kubectl edit configmap coredns -n kube-system",
			Notes:       "Change to: forward . /etc/resolv.conf   (or a real resolver such as 8.8.8.8)",
		},
		{
			Description: "Restart DNS pods",
			Command:     "kubectl delete pods -n kube-system -l k8s-app=kube-dns",
		},
		{
			Description: "Retest external DNS",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.google.com",
		},
	}
}
