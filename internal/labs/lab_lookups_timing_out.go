package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&LookupsTimingOutLab{})
}

type LookupsTimingOutLab struct {
	BaseLab
}

func (l *LookupsTimingOutLab) ID() string {
	return "lookups_timing_out"
}

func (l *LookupsTimingOutLab) Title() string {
	return "DNS Lookups Timing Out"
}

func (l *LookupsTimingOutLab) Category() Category {
	return CategoryDNS
}

func (l *LookupsTimingOutLab) Difficulty() Difficulty {
	return DifficultyEasy
}

func (l *LookupsTimingOutLab) Description() string {
	return `Pods cannot resolve any DNS names — lookups hang or time out.
Services and Deployments otherwise look healthy.

Your task: Restore DNS resolution for workloads in the cluster.`
}

func (l *LookupsTimingOutLab) Hints() []string {
	return []string{
		"Try nslookup from a debug pod and note whether it times out",
		"Check kube-system for the cluster DNS Deployment / pods",
		"Confirm the desired replica count is greater than zero",
		"Ensure the kube-dns / CoreDNS Service has Endpoints",
	}
}

func (l *LookupsTimingOutLab) EstimatedTime() int {
	return 15
}

func (l *LookupsTimingOutLab) Tags() []string {
	return []string{"dns", "workloads", "troubleshooting"}
}

func (l *LookupsTimingOutLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *LookupsTimingOutLab) Break(ctx context.Context, kubeconfigPath string) error {
	_, err := kubectl(ctx, kubeconfigPath, "scale", "deployment", "coredns",
		"-n", "kube-system", "--replicas=0")
	if err != nil {
		return fmt.Errorf("scaling DNS deployment: %w", err)
	}
	return nil
}

func (l *LookupsTimingOutLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	return nil
}

func (l *LookupsTimingOutLab) Verify(ctx context.Context, kubeconfigPath string) error {
	replicas, err := kubectl(ctx, kubeconfigPath, "get", "deployment", "coredns", "-n", "kube-system",
		"-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return fmt.Errorf("failed to check DNS deployment: %w", err)
	}
	if strings.TrimSpace(replicas) == "0" || strings.TrimSpace(replicas) == "" {
		return fmt.Errorf("DNS deployment still has 0 replicas")
	}

	if err := waitDNSPodsReady(ctx, kubeconfigPath); err != nil {
		return err
	}

	_, err = kubectl(ctx, kubeconfigPath, "run", "dns-verify-timeout", "--image=busybox:1.28",
		"--rm", "-i", "--restart=Never", "--", "nslookup", "kubernetes.default.svc.cluster.local")
	if err != nil {
		return fmt.Errorf("DNS lookup still failing: %w", err)
	}
	return nil
}

func (l *LookupsTimingOutLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the failure",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default",
		},
		{
			Description: "Check DNS workloads in kube-system",
			Command:     "kubectl get deploy,pods -n kube-system | grep -i dns",
			Notes:       "coredns Deployment may be scaled to 0",
		},
		{
			Description: "Restore replicas",
			Command:     "kubectl scale deployment coredns -n kube-system --replicas=2",
		},
		{
			Description: "Wait for pods and retest",
			Command:     "kubectl get pods -n kube-system -l k8s-app=kube-dns -w",
		},
		{
			Description: "Verify DNS works",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
