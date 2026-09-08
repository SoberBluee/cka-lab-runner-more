package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&DNSPodsRestartingLab{})
}

type DNSPodsRestartingLab struct {
	BaseLab
}

func (l *DNSPodsRestartingLab) ID() string {
	return "dns_pods_restarting"
}

func (l *DNSPodsRestartingLab) Title() string {
	return "DNS Pods Restarting"
}

func (l *DNSPodsRestartingLab) Category() Category {
	return CategoryDNS
}

func (l *DNSPodsRestartingLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *DNSPodsRestartingLab) Description() string {
	return `Name resolution has stopped working across the whole cluster after a maintenance
window. The DNS pods in kube-system restart continuously and never reach a Ready state,
even though their configuration content is valid.

Your task: Get the DNS pods running again and restore name resolution.`
}

func (l *DNSPodsRestartingLab) Hints() []string {
	return []string{
		"kubectl -n kube-system get pods -l k8s-app=kube-dns -o wide shows the restart count",
		"Read the container logs of a restarting pod, including the previous instance with --previous",
		"The error is about a file the process cannot open, not about the config content",
		"Compare the Deployment's volume definition with the keys in the ConfigMap it mounts",
	}
}

func (l *DNSPodsRestartingLab) EstimatedTime() int {
	return 25
}

func (l *DNSPodsRestartingLab) Tags() []string {
	return []string{"dns", "coredns", "deployments", "volumes", "troubleshooting"}
}

func (l *DNSPodsRestartingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return waitDNSPodsReady(ctx, kubeconfigPath)
}

func (l *DNSPodsRestartingLab) Break(ctx context.Context, kubeconfigPath string) error {
	volume, err := kubectl(ctx, kubeconfigPath, "get", "deployment", "coredns", "-n", "kube-system",
		"-o", "jsonpath={.spec.template.spec.volumes[0].name}")
	if err != nil {
		return fmt.Errorf("reading CoreDNS volumes: %w", err)
	}
	name := strings.TrimSpace(volume)
	if name == "" {
		name = "config-volume"
	}

	patch := fmt.Sprintf(`{"spec":{"template":{"spec":{"volumes":[{"name":%q,"configMap":{"name":"coredns","items":[{"key":"Corefile","path":"Corefile.bak"}]}}]}}}}`, name)
	if output, err := kubectl(ctx, kubeconfigPath, "patch", "deployment", "coredns", "-n", "kube-system",
		"-p", patch); err != nil {
		return fmt.Errorf("patching CoreDNS volume: %s: %w", output, err)
	}
	return nil
}

func (l *DNSPodsRestartingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	return waitDNSPodsUnready(ctx, kubeconfigPath)
}

func (l *DNSPodsRestartingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if err := waitDNSPodsReady(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("DNS pods are still not Ready: %w", err)
	}

	return waitFor(ctx, 60*time.Second, func() error {
		output, err := dnsLookupFromTempPod(ctx, kubeconfigPath, "dns-verify-restart",
			"kubernetes.default.svc.cluster.local")
		if err != nil {
			return fmt.Errorf("Service DNS lookup failed: %w", err)
		}
		if !dnsLookupAnswered(output) {
			return fmt.Errorf("lookup returned no answer: %s", output)
		}
		return nil
	})
}

func (l *DNSPodsRestartingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Look at the DNS pods",
			Command:     "kubectl get pods -n kube-system -l k8s-app=kube-dns",
			Notes:       "They restart repeatedly and never become Ready",
		},
		{
			Description: "Read the crash reason from the logs",
			Command:     "kubectl logs -n kube-system -l k8s-app=kube-dns --previous --tail=20",
			Notes:       "CoreDNS reports it cannot open /etc/coredns/Corefile",
		},
		{
			Description: "Check the ConfigMap content is fine",
			Command:     "kubectl get configmap coredns -n kube-system -o yaml",
			Notes:       "The Corefile key exists and the config is valid, so the content is not the problem",
		},
		{
			Description: "Inspect how the Deployment mounts that ConfigMap",
			Command:     "kubectl get deployment coredns -n kube-system -o jsonpath='{.spec.template.spec.volumes[0]}' | jq",
			Notes:       "The item is projected to path Corefile.bak, so no file called Corefile ever appears in the mount",
		},
		{
			Description: "Point the volume item back at the Corefile path",
			Command:     "kubectl patch deployment coredns -n kube-system -p '{\"spec\":{\"template\":{\"spec\":{\"volumes\":[{\"name\":\"config-volume\",\"configMap\":{\"name\":\"coredns\",\"items\":[{\"key\":\"Corefile\",\"path\":\"Corefile\"}]}}]}}}}'",
			Notes:       "kubectl -n kube-system edit deployment coredns works just as well",
		},
		{
			Description: "Wait for the rollout and retest",
			Command:     "kubectl rollout status deployment coredns -n kube-system && kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
