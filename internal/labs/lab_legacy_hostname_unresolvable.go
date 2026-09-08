package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&LegacyHostnameUnresolvableLab{})
}

type LegacyHostnameUnresolvableLab struct {
	BaseLab
}

func (l *LegacyHostnameUnresolvableLab) ID() string {
	return "legacy_hostname_unresolvable"
}

func (l *LegacyHostnameUnresolvableLab) Title() string {
	return "Legacy Hostname Unresolvable"
}

func (l *LegacyHostnameUnresolvableLab) Category() Category {
	return CategoryDNS
}

func (l *LegacyHostnameUnresolvableLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *LegacyHostnameUnresolvableLab) Description() string {
	return `An application being migrated into the cluster connects to the hostname
db.legacy.local, which no upstream DNS server knows about. The database is reachable at
10.96.240.10.

Your task: Configure cluster DNS so that every pod resolves db.legacy.local to 10.96.240.10.
Existing Service resolution and external name resolution must keep working.`
}

func (l *LegacyHostnameUnresolvableLab) Hints() []string {
	return []string{
		"Nothing is broken here — this is a configuration task on the cluster DNS server",
		"The DNS server config lives in the coredns ConfigMap in kube-system",
		"CoreDNS has a hosts plugin that serves static A records from the Corefile",
		"Any plugin that answers a name will stop the query there unless you allow it to fall through",
		"CoreDNS reloads by itself after a while; a rollout restart applies the change immediately",
	}
}

func (l *LegacyHostnameUnresolvableLab) EstimatedTime() int {
	return 25
}

func (l *LegacyHostnameUnresolvableLab) Tags() []string {
	return []string{"dns", "coredns", "corefile", "configuration"}
}

func (l *LegacyHostnameUnresolvableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return waitDNSPodsReady(ctx, kubeconfigPath)
}

func (l *LegacyHostnameUnresolvableLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: migration
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: legacy-client
  namespace: migration
spec:
  replicas: 1
  selector:
    matchLabels:
      app: legacy-client
  template:
    metadata:
      labels:
        app: legacy-client
    spec:
      containers:
      - name: client
        image: busybox:1.28
        command: ["sh", "-c", "sleep 3600"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying migration scenario: %w", err)
	}
	return nil
}

func (l *LegacyHostnameUnresolvableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	output, err := dnsLookupFromTempPod(ctx, kubeconfigPath, "dns-broken-legacy", "db.legacy.local")
	if err == nil && dnsLookupAnswered(output) {
		return fmt.Errorf("db.legacy.local already resolves")
	}
	return nil
}

func (l *LegacyHostnameUnresolvableLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if err := waitDNSPodsReady(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("DNS pods are not Ready: %w", err)
	}

	if err := waitFor(ctx, 45*time.Second, func() error {
		output, err := dnsLookupFromTempPod(ctx, kubeconfigPath, "dns-verify-legacy", "db.legacy.local")
		if err != nil {
			return fmt.Errorf("db.legacy.local does not resolve: %w", err)
		}
		if !strings.Contains(output, "10.96.240.10") {
			return fmt.Errorf("db.legacy.local did not resolve to 10.96.240.10: %s", output)
		}
		return nil
	}); err != nil {
		return err
	}

	return waitFor(ctx, 30*time.Second, func() error {
		output, err := dnsLookupFromTempPod(ctx, kubeconfigPath, "dns-verify-legacy-cluster",
			"kubernetes.default.svc.cluster.local")
		if err != nil {
			return fmt.Errorf("cluster Service resolution regressed — the new config must not swallow other queries: %w", err)
		}
		if !dnsLookupAnswered(output) {
			return fmt.Errorf("cluster Service lookup returned no answer: %s", output)
		}
		return nil
	})
}

func (l *LegacyHostnameUnresolvableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the name does not resolve today",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup db.legacy.local",
		},
		{
			Description: "Open the CoreDNS configuration",
			Command:     "kubectl -n kube-system edit configmap coredns",
		},
		{
			Description: "Add a hosts block inside the .:53 server block, above forward",
			Command:     "# Corefile",
			Notes: `hosts {
    10.96.240.10 db.legacy.local
    fallthrough
}

fallthrough is essential: without it the hosts plugin answers NXDOMAIN for every
other name and you break cluster and external resolution.`,
		},
		{
			Description: "Apply the change immediately",
			Command:     "kubectl -n kube-system rollout restart deployment coredns",
			Notes:       "The reload plugin also picks it up on its own, but that can take a couple of minutes",
		},
		{
			Description: "Verify the new record",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup db.legacy.local",
		},
		{
			Description: "Verify nothing else regressed",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
