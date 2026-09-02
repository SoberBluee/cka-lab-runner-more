package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&ServiceDiscoveryBrokenLab{})
}

type ServiceDiscoveryBrokenLab struct {
	BaseLab
}

func (l *ServiceDiscoveryBrokenLab) ID() string {
	return "service_discovery_broken"
}

func (l *ServiceDiscoveryBrokenLab) Title() string {
	return "Service Discovery Broken"
}

func (l *ServiceDiscoveryBrokenLab) Category() Category {
	return CategoryDNS
}

func (l *ServiceDiscoveryBrokenLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *ServiceDiscoveryBrokenLab) Description() string {
	return `Applications in the cluster cannot resolve Kubernetes Service DNS names
(for example kubernetes.default.svc.cluster.local). External name resolution may still work.

Your task: Restore Service name resolution in the cluster.`
}

func (l *ServiceDiscoveryBrokenLab) Hints() []string {
	return []string{
		"From a test pod, try nslookup against a Service DNS name and note the error",
		"Check which components in kube-system provide cluster DNS",
		"Inspect the DNS server configuration for the cluster domain / kubernetes plugin",
		"After fixing config, restart the DNS pods so they reload",
	}
}

func (l *ServiceDiscoveryBrokenLab) EstimatedTime() int {
	return 20
}

func (l *ServiceDiscoveryBrokenLab) Tags() []string {
	return []string{"dns", "services", "troubleshooting"}
}

func (l *ServiceDiscoveryBrokenLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ServiceDiscoveryBrokenLab) Break(ctx context.Context, kubeconfigPath string) error {
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
        kubernetes wrong.local in-addr.arpa ip6.arpa {
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
	if err := kubectlApply(ctx, kubeconfigPath, broken); err != nil {
		return fmt.Errorf("applying broken DNS config: %w", err)
	}
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pods", "-n", "kube-system", "-l", "k8s-app=kube-dns", "--force", "--grace-period=0")
	return nil
}

func (l *ServiceDiscoveryBrokenLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(15 * time.Second)
	return nil
}

func (l *ServiceDiscoveryBrokenLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if err := waitDNSPodsReady(ctx, kubeconfigPath); err != nil {
		return err
	}

	output, err := kubectl(ctx, kubeconfigPath, "run", "dns-verify-svc", "--image=busybox:1.28",
		"--rm", "-i", "--restart=Never", "--", "nslookup", "kubernetes.default.svc.cluster.local")
	if err != nil {
		return fmt.Errorf("service DNS lookup failed: %w", err)
	}
	if !strings.Contains(strings.ToLower(output), "address") && !strings.Contains(output, "Name:") {
		return fmt.Errorf("unexpected nslookup output: %s", output)
	}
	return nil
}

func (l *ServiceDiscoveryBrokenLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the failure",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
		{
			Description: "Check DNS pods",
			Command:     "kubectl get pods -n kube-system -l k8s-app=kube-dns",
		},
		{
			Description: "Inspect the DNS ConfigMap",
			Command:     "kubectl get configmap coredns -n kube-system -o yaml",
			Notes:       "The kubernetes plugin zone is wrong.local instead of cluster.local",
		},
		{
			Description: "Fix the kubernetes zone to cluster.local",
			Command:     "kubectl edit configmap coredns -n kube-system",
			Notes:       "Change 'kubernetes wrong.local' to 'kubernetes cluster.local'",
		},
		{
			Description: "Restart DNS pods",
			Command:     "kubectl delete pods -n kube-system -l k8s-app=kube-dns",
		},
		{
			Description: "Retest Service DNS",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
