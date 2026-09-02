package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&CatalogLookupRefusedLab{})
}

type CatalogLookupRefusedLab struct {
	BaseLab
}

func (l *CatalogLookupRefusedLab) ID() string {
	return "catalog_lookup_refused"
}

func (l *CatalogLookupRefusedLab) Title() string {
	return "Catalog Lookups Fail Right After A Rename"
}

func (l *CatalogLookupRefusedLab) Category() Category {
	return CategoryNetworking
}

func (l *CatalogLookupRefusedLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *CatalogLookupRefusedLab) Description() string {
	return `A tidy-up commit renamed a few things in the catalog manifests. Since it merged,
calls to http://catalog.catalog.svc.cluster.local fail immediately.

The catalog pod is Running and Ready, the Service selector matches it, and the pod serves
HTTP on port 80 — you can confirm that by calling the pod's IP directly.

Your task: get the call through the Service working again.`
}

func (l *CatalogLookupRefusedLab) Hints() []string {
	return []string{
		"kubectl get endpoints catalog -n catalog — compare the port shown there with the one the pod serves",
		"kubectl get svc catalog -n catalog -o yaml — look closely at targetPort",
		"targetPort may be a number or the *name* of a container port, and the name has to exist on the container",
		"kubectl get pod -n catalog -l app=catalog -o jsonpath='{.items[0].spec.containers[0].ports}'",
		"Either rename the container port to match the Service or point the Service at the name the container uses",
	}
}

func (l *CatalogLookupRefusedLab) EstimatedTime() int {
	return 15
}

func (l *CatalogLookupRefusedLab) Tags() []string {
	return []string{"networking", "services", "endpoints", "troubleshooting"}
}

func (l *CatalogLookupRefusedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *CatalogLookupRefusedLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: catalog
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: catalog
spec:
  replicas: 1
  selector:
    matchLabels:
      app: catalog
  template:
    metadata:
      labels:
        app: catalog
    spec:
      containers:
      - name: catalog
        image: nginx:alpine
        ports:
        - name: web
          containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: catalog
  namespace: catalog
spec:
  selector:
    app: catalog
  ports:
  - port: 80
    targetPort: http
---
apiVersion: v1
kind: Pod
metadata:
  name: shopper
  namespace: catalog
  labels:
    app: shopper
spec:
  containers:
  - name: shopper
    image: busybox:1.28
    command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying catalog scenario: %w", err)
	}
	return deploymentReady(ctx, kubeconfigPath, "catalog", "catalog", 1, 120*time.Second)
}

func (l *CatalogLookupRefusedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)

	if _, err := httpGetFromPod(ctx, kubeconfigPath, "catalog", "shopper",
		"http://catalog.catalog.svc.cluster.local", 5); err == nil {
		return fmt.Errorf("the catalog Service is already reachable")
	}
	return nil
}

func (l *CatalogLookupRefusedLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if err := waitFor(ctx, 30*time.Second, func() error {
		ports, err := endpointPorts(ctx, kubeconfigPath, "catalog", "catalog")
		if err != nil {
			return err
		}
		for _, port := range ports {
			if port == 80 {
				return nil
			}
		}
		return fmt.Errorf("catalog endpoints resolve to ports %v, expected 80", ports)
	}); err != nil {
		return err
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "catalog", "shopper",
			"http://catalog.catalog.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("the shopper pod still cannot reach the catalog Service: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from catalog: %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (l *CatalogLookupRefusedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the pod itself serves traffic",
			Command: `POD_IP=$(kubectl get pod -n catalog -l app=catalog -o jsonpath='{.items[0].status.podIP}')
kubectl exec -n catalog shopper -- wget -qO- --timeout=3 http://$POD_IP | head -3`,
			Notes: "Works — so the problem is between the Service and the pod, not in the pod",
		},
		{
			Description: "Compare Service and endpoints",
			Command:     "kubectl get svc catalog -n catalog -o yaml | grep -A3 ports; kubectl get endpoints catalog -n catalog",
			Notes:       "targetPort is the name 'http', which no container port is called",
		},
		{
			Description: "Check the container's port names",
			Command:     "kubectl get pod -n catalog -l app=catalog -o jsonpath='{.items[0].spec.containers[0].ports}'",
			Notes:       "The port is named 'web'",
		},
		{
			Description: "Fix the Service (option A: use the number)",
			Command:     `kubectl patch svc catalog -n catalog --type json -p='[{"op":"replace","path":"/spec/ports/0/targetPort","value":80}]'`,
		},
		{
			Description: "Fix the Service (option B: use the real name)",
			Command:     `kubectl patch svc catalog -n catalog --type json -p='[{"op":"replace","path":"/spec/ports/0/targetPort","value":"web"}]'`,
		},
		{
			Description: "Confirm endpoints and the call",
			Command:     "kubectl get endpoints catalog -n catalog && kubectl exec -n catalog shopper -- wget -qO- --timeout=3 http://catalog.catalog.svc.cluster.local | head -3",
		},
	}
}
