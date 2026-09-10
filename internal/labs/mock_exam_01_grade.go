package labs

import (
	"context"
	"sort"
	"strings"
)

func (l *MockExam01Lab) Grade(ctx context.Context, kubeconfigPath string) ExamReport {
	node, _ := getControlPlaneNode(ctx, kubeconfigPath)
	return ExamReport{Tasks: []ExamTaskResult{
		gradeMockTask1(ctx, kubeconfigPath),
		gradeMockTask2(ctx, node),
		gradeMockTask3(ctx, node),
		gradeMockTask4(ctx, kubeconfigPath),
		gradeMockTask5(ctx, kubeconfigPath),
		gradeMockTask6(ctx, kubeconfigPath),
		gradeMockTask7(ctx, kubeconfigPath),
		gradeMockTask8(ctx, kubeconfigPath),
		gradeMockTask9(ctx, kubeconfigPath),
		gradeMockTask10(ctx, kubeconfigPath),
		gradeMockTask11(ctx, kubeconfigPath),
		gradeMockTask12(ctx, kubeconfigPath),
	}}
}

func gradeMockTask1(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	env := kubeEquals(ctx, kubeconfigPath, "spec.nodeName", "get", "pod", "mc-pod", "-n", "mc-namespace",
		"-o", `jsonpath={.spec.containers[?(@.name=="mc-pod-1")].env[?(@.name=="NODE_NAME")].valueFrom.fieldRef.fieldPath}`)
	volume := kubeEquals(ctx, kubeconfigPath, "{}", "get", "pod", "mc-pod", "-n", "mc-namespace",
		"-o", `jsonpath={.spec.volumes[?(@.name=="shared-volume")].emptyDir}`)
	writerMount := kubeEquals(ctx, kubeconfigPath, "/var/log/shared", "get", "pod", "mc-pod", "-n", "mc-namespace",
		"-o", `jsonpath={.spec.containers[?(@.name=="mc-pod-2")].volumeMounts[?(@.name=="shared-volume")].mountPath}`)
	sidecarMount := kubeEquals(ctx, kubeconfigPath, "/var/log/shared", "get", "pod", "mc-pod", "-n", "mc-namespace",
		"-o", `jsonpath={.spec.containers[?(@.name=="mc-pod-3")].volumeMounts[?(@.name=="shared-volume")].mountPath}`)
	logs, err := kubectl(ctx, kubeconfigPath, "logs", "mc-pod", "-n", "mc-namespace", "-c", "mc-pod-3", "--tail=1")
	return examTask(1, "Shared-volume Pod", 8,
		examCheck("NODE_NAME uses the Downward API", 2, env),
		examCheck("shared-volume is an emptyDir", 2, volume),
		examCheck("writer and sidecar share the volume and the sidecar emits logs", 4,
			writerMount && sidecarMount && err == nil && strings.TrimSpace(logs) != ""))
}

func gradeMockTask2(ctx context.Context, node string) ExamTaskResult {
	config, configErr := dockerExec(ctx, node, "cat", "/etc/crictl.yaml")
	_, infoErr := dockerExec(ctx, node, "crictl", "info")
	correct := configErr == nil &&
		strings.Contains(config, "runtime-endpoint: unix:///run/containerd/containerd.sock") &&
		strings.Contains(config, "image-endpoint: unix:///run/containerd/containerd.sock")
	return examTask(2, "Node CRI configuration", 7,
		examCheck("crictl endpoints target containerd", 3, correct),
		examCheck("crictl can query the runtime", 4, infoErr == nil))
}

func gradeMockTask3(ctx context.Context, node string) ExamTaskResult {
	output, err := dockerExec(ctx, node, "cat", "/root/vpa-crds.txt")
	got := strings.Fields(output)
	sort.Strings(got)
	want := []string{
		"verticalpodautoscalercheckpoints.autoscaling.k8s.io",
		"verticalpodautoscalers.autoscaling.k8s.io",
	}
	return examTask(3, "VPA CRD inventory file", 6,
		examCheck("/root/vpa-crds.txt contains exactly the VPA CRDs", 6,
			err == nil && strings.Join(got, "\n") == strings.Join(want, "\n")))
}

func gradeMockTask4(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	exists := kubeEquals(ctx, kubeconfigPath, "ClusterIP 6379", "get", "service", "messaging-service",
		"-o", "jsonpath={.spec.type} {.spec.ports[0].port}")
	selector := kubeEquals(ctx, kubeconfigPath, "messaging", "get", "service", "messaging-service",
		"-o", "jsonpath={.spec.selector.app}")
	endpoints, _ := endpointAddressCount(ctx, kubeconfigPath, "default", "messaging-service")
	return examTask(4, "Messaging Service", 8,
		examCheck("Service is ClusterIP on port 6379", 4, exists),
		examCheck("selector produces a backend endpoint", 4, selector && endpoints > 0))
}

func gradeMockTask5(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "kodekloud/webapp-color", "get", "deployment", "hr-web-app",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	replicas := kubeEquals(ctx, kubeconfigPath, "2", "get", "deployment", "hr-web-app",
		"-o", "jsonpath={.spec.replicas}")
	ready := kubeEquals(ctx, kubeconfigPath, "2", "get", "deployment", "hr-web-app",
		"-o", "jsonpath={.status.readyReplicas}")
	return examTask(5, "HR web Deployment", 10,
		examCheck("Deployment uses the requested image", 4, image),
		examCheck("Deployment requests two replicas", 3, replicas),
		examCheck("both replicas are Ready", 3, ready))
}

func gradeMockTask6(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	ready := kubeEquals(ctx, kubeconfigPath, "True", "get", "pod", "orange",
		"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
	return examTask(6, "Orange application", 12,
		examCheck("orange Pod is Ready", 12, ready))
}

func gradeMockTask7(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	service := kubeEquals(ctx, kubeconfigPath, "NodePort 30082 8080", "get", "service", "hr-web-app-service",
		"-o", "jsonpath={.spec.type} {.spec.ports[0].nodePort} {.spec.ports[0].targetPort}")
	port := kubeEquals(ctx, kubeconfigPath, "8080", "get", "service", "hr-web-app-service",
		"-o", "jsonpath={.spec.ports[0].port}")
	endpoints, _ := endpointAddressCount(ctx, kubeconfigPath, "default", "hr-web-app-service")
	return examTask(7, "HR web NodePort Service", 8,
		examCheck("Service is NodePort 30082 targeting 8080", 5, service),
		examCheck("Service port is 8080 with backend endpoints", 3, port && endpoints > 0))
}

func gradeMockTask8(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	capacity := kubeEquals(ctx, kubeconfigPath, "100Mi", "get", "pv", "pv-analytics",
		"-o", "jsonpath={.spec.capacity.storage}")
	access := kubeEquals(ctx, kubeconfigPath, "ReadWriteMany", "get", "pv", "pv-analytics",
		"-o", "jsonpath={.spec.accessModes[0]}")
	path := kubeEquals(ctx, kubeconfigPath, "/pv/data-analytics", "get", "pv", "pv-analytics",
		"-o", "jsonpath={.spec.hostPath.path}")
	return examTask(8, "Analytics PersistentVolume", 8,
		examCheck("capacity is 100Mi", 2, capacity),
		examCheck("access mode is ReadWriteMany", 2, access),
		examCheck("hostPath is /pv/data-analytics", 4, path))
}

func gradeMockTask9(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	target := kubeEquals(ctx, kubeconfigPath, "apps/v1 Deployment kkapp-deploy 2 10", "get", "hpa", "webapp-hpa",
		"-o", "jsonpath={.spec.scaleTargetRef.apiVersion} {.spec.scaleTargetRef.kind} {.spec.scaleTargetRef.name} {.spec.minReplicas} {.spec.maxReplicas}")
	cpu := kubeEquals(ctx, kubeconfigPath, "Resource cpu Utilization 50", "get", "hpa", "webapp-hpa",
		"-o", "jsonpath={.spec.metrics[0].type} {.spec.metrics[0].resource.name} {.spec.metrics[0].resource.target.type} {.spec.metrics[0].resource.target.averageUtilization}")
	window := kubeEquals(ctx, kubeconfigPath, "300", "get", "hpa", "webapp-hpa",
		"-o", "jsonpath={.spec.behavior.scaleDown.stabilizationWindowSeconds}")
	return examTask(9, "Horizontal Pod Autoscaler", 10,
		examCheck("HPA targets kkapp-deploy with bounds 2–10", 3, target),
		examCheck("CPU utilization target is 50%", 4, cpu),
		examCheck("scale-down stabilization is 300 seconds", 3, window))
}

func gradeMockTask10(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	target := kubeEquals(ctx, kubeconfigPath, "apps/v1 Deployment analytics-deployment", "get",
		"verticalpodautoscaler", "analytics-vpa", "-o",
		"jsonpath={.spec.targetRef.apiVersion} {.spec.targetRef.kind} {.spec.targetRef.name}")
	mode := kubeEquals(ctx, kubeconfigPath, "Recreate", "get", "verticalpodautoscaler", "analytics-vpa",
		"-o", "jsonpath={.spec.updatePolicy.updateMode}")
	return examTask(10, "Vertical Pod Autoscaler", 9,
		examCheck("VPA targets analytics-deployment", 4, target),
		examCheck("update mode is Recreate", 5, mode))
}

func gradeMockTask11(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	class := kubeEquals(ctx, kubeconfigPath, "nginx", "get", "gateway", "web-gateway",
		"-n", "nginx-gateway", "-o", "jsonpath={.spec.gatewayClassName}")
	listener := kubeEquals(ctx, kubeconfigPath, "http HTTP 80", "get", "gateway", "web-gateway",
		"-n", "nginx-gateway", "-o", "jsonpath={.spec.listeners[0].name} {.spec.listeners[0].protocol} {.spec.listeners[0].port}")
	return examTask(11, "Gateway resource", 6,
		examCheck("Gateway uses class nginx", 2, class),
		examCheck("http listener uses HTTP on port 80", 4, listener))
}

func gradeMockTask12(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	releases, err := runHostCommand(ctx, "helm", "list", "-n", "kk-ns", "-f", "^kk-mock1$",
		"-o", "json", "--kubeconfig", kubeconfigPath)
	version := err == nil && strings.Contains(releases, `"chart":"podinfo-6.11.2"`)
	ready := kubeEquals(ctx, kubeconfigPath, "1", "get", "deployment", "kk-mock1-podinfo",
		"-n", "kk-ns", "-o", "jsonpath={.status.readyReplicas}")
	return examTask(12, "Helm release upgrade", 8,
		examCheck("kk-mock1 uses chart version 6.11.2", 5, version),
		examCheck("release Deployment is Ready", 3, ready))
}

func examTask(number int, title string, weight int, checks ...ExamCheckResult) ExamTaskResult {
	return ExamTaskResult{Number: number, Title: title, Weight: weight, Checks: checks}
}

func examCheck(description string, points int, passed bool) ExamCheckResult {
	return ExamCheckResult{Description: description, Points: points, Passed: passed}
}

func kubeEquals(ctx context.Context, kubeconfigPath, want string, args ...string) bool {
	output, err := kubectl(ctx, kubeconfigPath, args...)
	return err == nil && strings.TrimSpace(output) == want
}
