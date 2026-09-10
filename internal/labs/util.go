package labs

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// kubectl executes a kubectl command with the given kubeconfig
func kubectl(ctx context.Context, kubeconfigPath string, args ...string) (string, error) {
	fullArgs := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// kubectlApply applies a YAML manifest
func kubectlApply(ctx context.Context, kubeconfigPath, yaml string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %s: %w", string(output), err)
	}
	return nil
}

// getControlPlaneNode returns the name of the control plane node
func getControlPlaneNode(ctx context.Context, kubeconfigPath string) (string, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "nodes",
		"-l", "node-role.kubernetes.io/control-plane",
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("getting control plane node: %w", err)
	}

	nodeName := strings.TrimSpace(output)
	if nodeName == "" {
		return "", fmt.Errorf("no control plane node found")
	}

	return nodeName, nil
}

// dockerExec executes a command inside a docker container (for kind nodes)
func dockerExec(ctx context.Context, containerName string, args ...string) (string, error) {
	fullArgs := append([]string{"exec", containerName}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// dockerCp copies a file to/from a docker container
func dockerCp(ctx context.Context, src, dst string) error {
	cmd := exec.CommandContext(ctx, "docker", "cp", src, dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp failed: %s: %w", string(output), err)
	}
	return nil
}

// WaitForClusterReady waits for the cluster to be ready by checking if nodes are accessible
func WaitForClusterReady(ctx context.Context, kubeconfigPath string) error {
	for i := 0; i < 30; i++ {
		_, err := kubectl(ctx, kubeconfigPath, "get", "nodes")
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("cluster did not become ready in time")
}

// waitFor polls check until it succeeds or the timeout elapses, returning the
// last error seen.
func waitFor(ctx context.Context, timeout time.Duration, check func() error) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		lastErr = check()
		if lastErr == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// nodeNames returns every node name in the cluster
func nodeNames(ctx context.Context, kubeconfigPath string) ([]string, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	names := strings.Fields(output)
	if len(names) == 0 {
		return nil, fmt.Errorf("no nodes found")
	}
	return names, nil
}

// podNameByLabel returns the first pod name matching a label selector
func podNameByLabel(ctx context.Context, kubeconfigPath, namespace, selector string) (string, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", namespace,
		"-l", selector, "--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("finding pod %s in %s: %w", selector, namespace, err)
	}
	name := strings.TrimSpace(output)
	if name == "" {
		return "", fmt.Errorf("no running pod matching %s in namespace %s", selector, namespace)
	}
	return name, nil
}

// nodeHasTaintKey reports whether a node still carries a taint with the given key
func nodeHasTaintKey(ctx context.Context, kubeconfigPath, node, key string) bool {
	output, err := kubectl(ctx, kubeconfigPath, "get", "node", node,
		"-o", "jsonpath={.spec.taints[*].key}")
	if err != nil {
		return false
	}
	for _, k := range strings.Fields(output) {
		if k == key {
			return true
		}
	}
	return false
}

// authCanI runs kubectl auth can-i as another subject and reports the answer
func authCanI(ctx context.Context, kubeconfigPath, as, verb, resource, namespace string) bool {
	args := []string{"auth", "can-i", verb, resource, "--as=" + as}
	if namespace == "" {
		args = append(args, "--all-namespaces")
	} else {
		args = append(args, "-n", namespace)
	}
	output, _ := kubectl(ctx, kubeconfigPath, args...)
	// kubectl may print a warning line before the answer
	lines := strings.Fields(strings.TrimSpace(output))
	if len(lines) == 0 {
		return false
	}
	return lines[len(lines)-1] == "yes"
}

// staticPodImage returns the container image of a control plane static pod
// (component is e.g. kube-apiserver or kube-controller-manager)
func staticPodImage(ctx context.Context, kubeconfigPath, component string) (string, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
		"-l", "component="+component,
		"-o", "jsonpath={.items[0].spec.containers[0].image}")
	if err != nil {
		return "", fmt.Errorf("reading %s image: %w", component, err)
	}
	image := strings.TrimSpace(output)
	if image == "" {
		return "", fmt.Errorf("no %s static pod found", component)
	}
	return image, nil
}

// imageTag returns the tag portion of a container image reference
func imageTag(image string) string {
	idx := strings.LastIndex(image, ":")
	if idx == -1 || strings.Contains(image[idx:], "/") {
		return ""
	}
	return image[idx+1:]
}

// deploymentReady waits until a Deployment reports at least minReady available replicas
func deploymentReady(ctx context.Context, kubeconfigPath, namespace, name string, minReady int, timeout time.Duration) error {
	return waitFor(ctx, timeout, func() error {
		output, err := kubectl(ctx, kubeconfigPath, "get", "deployment", name, "-n", namespace,
			"-o", "jsonpath={.status.readyReplicas}")
		if err != nil {
			return fmt.Errorf("deployment %s/%s not found: %w", namespace, name, err)
		}
		ready, _ := strconv.Atoi(strings.TrimSpace(output))
		if ready < minReady {
			return fmt.Errorf("deployment %s/%s has %d ready replicas, want %d", namespace, name, ready, minReady)
		}
		return nil
	})
}

// daemonSetCounts returns desired and ready pod counts for a DaemonSet
func daemonSetCounts(ctx context.Context, kubeconfigPath, namespace, name string) (int, int, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "daemonset", name, "-n", namespace,
		"-o", "jsonpath={.status.desiredNumberScheduled} {.status.numberReady}")
	if err != nil {
		return 0, 0, fmt.Errorf("daemonset %s/%s not found: %w", namespace, name, err)
	}
	fields := strings.Fields(output)
	desired, ready := 0, 0
	if len(fields) > 0 {
		desired, _ = strconv.Atoi(fields[0])
	}
	if len(fields) > 1 {
		ready, _ = strconv.Atoi(fields[1])
	}
	return desired, ready, nil
}

// httpGetFromPod performs an HTTP GET from inside a pod using busybox wget
func httpGetFromPod(ctx context.Context, kubeconfigPath, namespace, pod, url string, timeoutSeconds int) (string, error) {
	return kubectl(ctx, kubeconfigPath, "exec", "-n", namespace, pod, "--",
		"wget", "-O-", "-q", fmt.Sprintf("--timeout=%d", timeoutSeconds), url)
}

// endpointSlices reads the EndpointSlices backing a Service. The older v1
// Endpoints API prints a deprecation warning on current clusters, which would
// end up mixed into jsonpath output, so slices are parsed as JSON instead.
func endpointSlices(ctx context.Context, kubeconfigPath, namespace, service string) ([]endpointSlice, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "endpointslices", "-n", namespace,
		"-l", "kubernetes.io/service-name="+service, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("reading endpoint slices for %s/%s: %w", namespace, service, err)
	}
	start := strings.Index(output, "{")
	if start == -1 {
		return nil, fmt.Errorf("unexpected output reading endpoint slices for %s/%s", namespace, service)
	}

	var list struct {
		Items []endpointSlice `json:"items"`
	}
	if err := json.Unmarshal([]byte(output[start:]), &list); err != nil {
		return nil, fmt.Errorf("parsing endpoint slices for %s/%s: %w", namespace, service, err)
	}
	return list.Items, nil
}

type endpointSlice struct {
	Ports []struct {
		Port *int `json:"port"`
	} `json:"ports"`
	Endpoints []struct {
		Addresses  []string `json:"addresses"`
		Conditions struct {
			Ready *bool `json:"ready"`
		} `json:"conditions"`
	} `json:"endpoints"`
}

// endpointAddressCount returns how many ready endpoint addresses back a Service
func endpointAddressCount(ctx context.Context, kubeconfigPath, namespace, service string) (int, error) {
	slices, err := endpointSlices(ctx, kubeconfigPath, namespace, service)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, slice := range slices {
		for _, endpoint := range slice.Endpoints {
			// A nil ready condition means "unknown", which callers treat as ready
			if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
				count += len(endpoint.Addresses)
			}
		}
	}
	return count, nil
}

// endpointPorts returns the resolved target ports a Service's endpoints listen on
func endpointPorts(ctx context.Context, kubeconfigPath, namespace, service string) ([]int, error) {
	slices, err := endpointSlices(ctx, kubeconfigPath, namespace, service)
	if err != nil {
		return nil, err
	}

	var ports []int
	for _, slice := range slices {
		for _, port := range slice.Ports {
			if port.Port != nil {
				ports = append(ports, *port.Port)
			}
		}
	}
	return ports, nil
}

// etcdPodName returns the name of the etcd static pod
func etcdPodName(ctx context.Context, kubeconfigPath string) (string, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
		"-l", "component=etcd", "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("finding etcd pod: %w", err)
	}
	name := strings.TrimSpace(output)
	if name == "" {
		return "", fmt.Errorf("no etcd static pod found")
	}
	return name, nil
}

// etcdExec runs a command inside the etcd static pod. The arguments are passed
// straight to exec rather than through a shell, because the etcd image cannot be
// relied on to ship one.
func etcdExec(ctx context.Context, kubeconfigPath string, args ...string) (string, error) {
	pod, err := etcdPodName(ctx, kubeconfigPath)
	if err != nil {
		return "", err
	}
	full := append([]string{"exec", "-n", "kube-system", pod, "--"}, args...)
	return kubectl(ctx, kubeconfigPath, full...)
}

// etcdPKIArgs are the client flags every etcdctl call against the cluster's
// datastore needs.
func etcdPKIArgs() []string {
	return []string{
		"--endpoints=https://127.0.0.1:2379",
		"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
		"--cert=/etc/kubernetes/pki/etcd/server.crt",
		"--key=/etc/kubernetes/pki/etcd/server.key",
	}
}

// waitForReadyPod returns a Ready pod matching the selector, skipping pods that
// are terminating so a verify run after a rollout does not inspect the old pod.
func waitForReadyPod(ctx context.Context, kubeconfigPath, namespace, selector string) (string, error) {
	var pod string
	err := waitFor(ctx, 90*time.Second, func() error {
		output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", namespace, "-l", selector,
			"--field-selector=status.phase=Running", "--no-headers",
			"-o", "custom-columns=NAME:.metadata.name,DEL:.metadata.deletionTimestamp,READY:.status.containerStatuses[0].ready")
		if err != nil {
			return fmt.Errorf("listing pods %s in %s: %w", selector, namespace, err)
		}
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 3 && fields[1] == "<none>" && fields[2] == "true" {
				pod = fields[0]
				return nil
			}
		}
		return fmt.Errorf("no ready pod matching %s in namespace %s", selector, namespace)
	})
	if err != nil {
		return "", err
	}
	return pod, nil
}

// dnsLookupFromTempPod runs nslookup from a throwaway busybox pod. The pod name
// must be unique per lab so concurrent or repeated verifies cannot collide.
func dnsLookupFromTempPod(ctx context.Context, kubeconfigPath, podName, target string) (string, error) {
	// A verify that timed out leaves the pod behind, and kubectl run would then
	// fail with "already exists" instead of reporting the lookup result.
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pod", podName, "--ignore-not-found",
		"--force", "--grace-period=0")
	return kubectl(ctx, kubeconfigPath, "run", podName, "--image=busybox:1.28",
		"--rm", "-i", "--restart=Never", "--", "nslookup", target)
}

// dnsLookupAnswered reports whether busybox nslookup output contains an answer
// section. A failed lookup still prints the server address, so the presence of
// "Name:" is what distinguishes success from NXDOMAIN or a timeout.
func dnsLookupAnswered(output string) bool {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "can't resolve") || strings.Contains(lower, "no answer") {
		return false
	}
	return strings.Contains(output, "Name:")
}

// assertNoDebugPods fails if common temporary debug pod names remain in a namespace.
func assertNoDebugPods(ctx context.Context, kubeconfigPath, namespace string) error {
	blocked := map[string]bool{
		"tmp": true, "test": true, "debug": true, "curl": true,
		"wget": true, "client": true, "netshoot": true, "busybox": true,
	}
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", namespace,
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil
	}
	for _, name := range strings.Fields(output) {
		if blocked[name] {
			return fmt.Errorf("temporary debug pod %q still present in namespace %s — delete it", name, namespace)
		}
	}
	return nil
}

func waitDNSPodsReady(ctx context.Context, kubeconfigPath string) error {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		phases, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
			"-l", "k8s-app=kube-dns",
			"-o", "jsonpath={.items[*].status.phase}")
		if err == nil {
			fields := strings.Fields(phases)
			if len(fields) > 0 {
				allRunning := true
				for _, p := range fields {
					if p != "Running" {
						allRunning = false
						break
					}
				}
				if allRunning {
					ready, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
						"-l", "k8s-app=kube-dns",
						"-o", "jsonpath={.items[*].status.containerStatuses[*].ready}")
					if strings.Contains(ready, "true") && !strings.Contains(ready, "false") {
						return nil
					}
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("DNS pods not ready in time")
}
