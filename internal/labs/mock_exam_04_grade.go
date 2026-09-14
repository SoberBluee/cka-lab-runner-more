package labs

import (
	"context"
	"fmt"
	"strings"
)

func (l *MockExam04Lab) Grade(ctx context.Context, kubeconfigPath string) ExamReport {
	node, _ := getControlPlaneNode(ctx, kubeconfigPath)
	return ExamReport{Tasks: []ExamTaskResult{
		gradeMock04Task1(ctx, kubeconfigPath),
		gradeMock04Task2(ctx, kubeconfigPath),
		gradeMock04Task3(ctx, kubeconfigPath),
		gradeMock04Task4(ctx, kubeconfigPath),
		gradeMock04Task5(ctx, kubeconfigPath),
		gradeMock04Task6(ctx, kubeconfigPath, node),
		gradeMock04Task7(ctx, kubeconfigPath, node),
		gradeMock04Task8(ctx, kubeconfigPath),
		gradeMock04Task9(ctx, kubeconfigPath),
		gradeMock04Task10(ctx, kubeconfigPath),
		gradeMock04Task11(ctx, kubeconfigPath),
	}}
}

func gradeMock04Task1(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	provisioner := kubeEquals(ctx, kubeconfigPath, "kubernetes.io/no-provisioner",
		"get", "sc", "disk-local", "-o", "jsonpath={.provisioner}")
	binding := kubeEquals(ctx, kubeconfigPath, "WaitForFirstConsumer",
		"get", "sc", "disk-local", "-o", "jsonpath={.volumeBindingMode}")
	expand := kubeEquals(ctx, kubeconfigPath, "true",
		"get", "sc", "disk-local", "-o", "jsonpath={.allowVolumeExpansion}")
	def := kubeEquals(ctx, kubeconfigPath, "true",
		"get", "sc", "disk-local", "-o",
		`jsonpath={.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}`)
	return examTask(1, "Default StorageClass", 8,
		examCheck("StorageClass disk-local exists with no-provisioner", 2, provisioner),
		examCheck("volumeBindingMode is WaitForFirstConsumer", 2, binding),
		examCheck("volume expansion is enabled", 2, expand),
		examCheck("disk-local is the default StorageClass", 2, def))
}

func gradeMock04Task2(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	builder := kubeEquals(ctx, kubeconfigPath, "busybox:1.28", "get", "deploy", "content-publisher",
		"-n", "publish", "-o", `jsonpath={.spec.template.spec.containers[?(@.name=="builder")].image}`)
	web := kubeEquals(ctx, kubeconfigPath, "nginx:alpine", "get", "deploy", "content-publisher",
		"-n", "publish", "-o", `jsonpath={.spec.template.spec.containers[?(@.name=="web")].image}`)
	vol := kubeEquals(ctx, kubeconfigPath, "{}", "get", "deploy", "content-publisher",
		"-n", "publish", "-o", `jsonpath={.spec.template.spec.volumes[?(@.name=="site-data")].emptyDir}`)
	builderMount := kubeEquals(ctx, kubeconfigPath, "/content", "get", "deploy", "content-publisher",
		"-n", "publish", "-o",
		`jsonpath={.spec.template.spec.containers[?(@.name=="builder")].volumeMounts[?(@.name=="site-data")].mountPath}`)
	webMount := kubeEquals(ctx, kubeconfigPath, "/usr/share/nginx/html", "get", "deploy", "content-publisher",
		"-n", "publish", "-o",
		`jsonpath={.spec.template.spec.containers[?(@.name=="web")].volumeMounts[?(@.name=="site-data")].mountPath}`)
	ready := kubeEquals(ctx, kubeconfigPath, "1", "get", "deploy", "content-publisher",
		"-n", "publish", "-o", "jsonpath={.status.readyReplicas}")
	return examTask(2, "Shared content publisher", 10,
		examCheck("builder and web containers use the required images", 3, builder && web),
		examCheck("both containers mount emptyDir site-data at the required paths", 4, vol && builderMount && webMount),
		examCheck("Deployment has a Ready replica", 3, ready))
}

func gradeMock04Task3(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	host := kubeEquals(ctx, kubeconfigPath, "shop.exam.local", "get", "ingress", "portal-ingress",
		"-n", "edge", "-o", "jsonpath={.spec.rules[0].host}")
	path := kubeEquals(ctx, kubeconfigPath, "/ Prefix", "get", "ingress", "portal-ingress",
		"-n", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[0].path} {.spec.rules[0].http.paths[0].pathType}")
	backend := kubeEquals(ctx, kubeconfigPath, "portal-svc 80", "get", "ingress", "portal-ingress",
		"-n", "edge", "-o",
		"jsonpath={.spec.rules[0].http.paths[0].backend.service.name} {.spec.rules[0].http.paths[0].backend.service.port.number}")
	return examTask(3, "Portal Ingress", 10,
		examCheck("host is shop.exam.local", 3, host),
		examCheck("path / uses Prefix", 3, path),
		examCheck("backend is portal-svc:80", 4, backend))
}

func gradeMock04Task4(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "nginx:1.21", "get", "deploy", "web-rollout",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	genStr, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "web-rollout",
		"-o", "jsonpath={.metadata.generation}")
	gen := 0
	fmt.Sscanf(strings.TrimSpace(genStr), "%d", &gen)
	return examTask(4, "Rolling update", 8,
		examCheck("web-rollout uses nginx:1.21", 5, image),
		examCheck("Deployment was updated after creation (generation >= 2)", 3, gen >= 2))
}

func gradeMock04Task5(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	csrApproved := false
	cond, err := kubectl(ctx, kubeconfigPath, "get", "csr", "maria-developer",
		"-o", `jsonpath={.status.conditions[?(@.type=="Approved")].status}`)
	if err == nil && strings.TrimSpace(cond) == "True" {
		csrApproved = true
	}
	// Some clusters only expose certificate once approved
	if !csrApproved {
		cert, _ := kubectl(ctx, kubeconfigPath, "get", "csr", "maria-developer",
			"-o", "jsonpath={.status.certificate}")
		csrApproved = strings.TrimSpace(cert) != ""
	}

	roleRef := kubeEquals(ctx, kubeconfigPath, "Role/developer", "get", "rolebinding", "developer-binding",
		"-n", "team-dev", "-o", "jsonpath={.roleRef.kind}/{.roleRef.name}")
	subject := kubeEquals(ctx, kubeconfigPath, "User/maria", "get", "rolebinding", "developer-binding",
		"-n", "team-dev", "-o", "jsonpath={.subjects[0].kind}/{.subjects[0].name}")

	can := authCanI(ctx, kubeconfigPath, "maria", "create", "pods", "team-dev") &&
		authCanI(ctx, kubeconfigPath, "maria", "list", "pods", "team-dev") &&
		authCanI(ctx, kubeconfigPath, "maria", "get", "pods", "team-dev") &&
		authCanI(ctx, kubeconfigPath, "maria", "update", "pods", "team-dev") &&
		authCanI(ctx, kubeconfigPath, "maria", "delete", "pods", "team-dev")
	least := !authCanI(ctx, kubeconfigPath, "maria", "list", "secrets", "team-dev")

	return examTask(5, "User certificate and RBAC", 10,
		examCheck("CSR maria-developer is Approved", 3, csrApproved),
		examCheck("RoleBinding wires Role/developer to User/maria", 3, roleRef && subject),
		examCheck("maria can manage pods in team-dev (least privilege)", 4, can && least))
}

func gradeMock04Task6(ctx context.Context, kubeconfigPath string, node string) ExamTaskResult {
	pod := kubeEquals(ctx, kubeconfigPath, "Running", "get", "pod", "dns-probe",
		"-o", "jsonpath={.status.phase}")
	svc := kubeEquals(ctx, kubeconfigPath, "ClusterIP", "get", "svc", "dns-probe-svc",
		"-o", "jsonpath={.spec.type}")
	svcFile, svcErr := dockerExec(ctx, node, "cat", "/opt/CKA/dns.svc")
	podFile, podErr := dockerExec(ctx, node, "cat", "/opt/CKA/dns.pod")
	svcOK := svcErr == nil && (strings.Contains(svcFile, "dns-probe-svc") || strings.Contains(svcFile, "Name:"))
	podOK := podErr == nil && (strings.Contains(podFile, ".default.pod") || strings.Contains(podFile, "Name:"))
	return examTask(6, "DNS lookup artifacts", 10,
		examCheck("Pod dns-probe is Running and Service dns-probe-svc is ClusterIP", 3, pod && svc),
		examCheck("/opt/CKA/dns.svc has Service DNS lookup output", 3, svcOK),
		examCheck("/opt/CKA/dns.pod has Pod DNS lookup output", 4, podOK))
}

func gradeMock04Task7(ctx context.Context, kubeconfigPath string, node string) ExamTaskResult {
	_, fileErr := dockerExec(ctx, node, "test", "-f", "/etc/kubernetes/manifests/priority-web.yaml")
	podName := "priority-web-" + node
	phase := kubeEquals(ctx, kubeconfigPath, "Running", "get", "pod", podName,
		"-o", "jsonpath={.status.phase}")
	if !phase {
		// Mirror pods sometimes appear in a namespace; also try listing
		out, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-A",
			"-o", "jsonpath={range .items[*]}{.metadata.name}{' '}{.status.phase}{'\\n'}{end}")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "priority-web-") && strings.Contains(line, "Running") {
					phase = true
					break
				}
			}
		}
	}
	return examTask(7, "Static Pod", 8,
		examCheck("manifest exists under /etc/kubernetes/manifests", 3, fileErr == nil),
		examCheck("priority-web static Pod is Running", 5, phase))
}

func gradeMock04Task8(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	target := kubeEquals(ctx, kubeconfigPath, "apps/v1 Deployment worker-deploy 2 12",
		"get", "hpa", "worker-hpa", "-n", "api",
		"-o", "jsonpath={.spec.scaleTargetRef.apiVersion} {.spec.scaleTargetRef.kind} {.spec.scaleTargetRef.name} {.spec.minReplicas} {.spec.maxReplicas}")
	memory := kubeEquals(ctx, kubeconfigPath, "Resource memory Utilization 70",
		"get", "hpa", "worker-hpa", "-n", "api",
		"-o", "jsonpath={.spec.metrics[0].type} {.spec.metrics[0].resource.name} {.spec.metrics[0].resource.target.type} {.spec.metrics[0].resource.target.averageUtilization}")
	return examTask(8, "Horizontal Pod Autoscaler", 10,
		examCheck("HPA targets worker-deploy with bounds 2–12", 5, target),
		examCheck("memory utilization target is 70%", 5, memory))
}

func gradeMock04Task9(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	class := kubeEquals(ctx, kubeconfigPath, "cka-gateway", "get", "gateway", "edge-gateway",
		"-n", "edge-gw", "-o", "jsonpath={.spec.gatewayClassName}")
	listener := kubeEquals(ctx, kubeconfigPath, "https HTTPS 443", "get", "gateway", "edge-gateway",
		"-n", "edge-gw",
		"-o", "jsonpath={.spec.listeners[0].name} {.spec.listeners[0].protocol} {.spec.listeners[0].port}")
	host := kubeEquals(ctx, kubeconfigPath, "shop.exam.local", "get", "gateway", "edge-gateway",
		"-n", "edge-gw", "-o", "jsonpath={.spec.listeners[0].hostname}")
	cert := kubeEquals(ctx, kubeconfigPath, "shop-tls", "get", "gateway", "edge-gateway",
		"-n", "edge-gw", "-o", "jsonpath={.spec.listeners[0].tls.certificateRefs[0].name}")
	return examTask(9, "Gateway TLS listener", 10,
		examCheck("GatewayClass unchanged", 2, class),
		examCheck("https listener is HTTPS on port 443", 3, listener),
		examCheck("hostname shop.exam.local with Secret shop-tls", 5, host && cert))
}

func gradeMock04Task10(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	legacyGone := true
	if out, err := helm(ctx, kubeconfigPath, "list", "-n", "charts-legacy", "-o", "json"); err == nil {
		if strings.Contains(out, "legacy-color") {
			legacyGone = false
		}
	}
	// Ensure vulnerable image is not still running via leftover release objects
	deploys, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "-A",
		"-o", "jsonpath={range .items[*]}{.metadata.namespace}/{.metadata.name}={.spec.template.spec.containers[0].image}{'\\n'}{end}")
	vulnGone := !strings.Contains(deploys, "nginx:1.14-alpine")

	safeOK := false
	if out, err := helm(ctx, kubeconfigPath, "list", "-n", "charts-safe", "-o", "json"); err == nil {
		safeOK = strings.Contains(out, "safe-portal") && strings.Contains(out, "docs-site")
	}
	return examTask(10, "Remove vulnerable Helm release", 10,
		examCheck("legacy-color release is uninstalled", 5, legacyGone && vulnGone),
		examCheck("unrelated Helm releases remain installed", 5, safeOK))
}

func gradeMock04Task11(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	correct := kubeEquals(ctx, kubeconfigPath, "net-policy-3", "get", "netpol", "net-policy-3",
		"-n", "backend", "-o", "jsonpath={.metadata.name}")
	_, bad1 := kubectl(ctx, kubeconfigPath, "get", "netpol", "net-policy-1", "-n", "backend")
	_, bad2 := kubectl(ctx, kubeconfigPath, "get", "netpol", "net-policy-2", "-n", "backend")
	return examTask(11, "NetworkPolicy selection", 6,
		examCheck("correct NetworkPolicy net-policy-3 is applied", 3, correct),
		examCheck("incorrect NetworkPolicies were not applied", 3, bad1 != nil && bad2 != nil))
}
