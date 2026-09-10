package labs

import (
	"context"
	"sort"
	"strings"
)

func (l *MockExam02Lab) Grade(ctx context.Context, kubeconfigPath string) ExamReport {
	node, _ := getControlPlaneNode(ctx, kubeconfigPath)
	return ExamReport{Tasks: []ExamTaskResult{
		gradeMock02Task1(ctx, kubeconfigPath),
		gradeMock02Task2(ctx, kubeconfigPath),
		gradeMock02Task3(ctx, kubeconfigPath),
		gradeMock02Task4(ctx, kubeconfigPath),
		gradeMock02Task5(ctx, node),
		gradeMock02Task6(ctx, kubeconfigPath),
		gradeMock02Task7(ctx, kubeconfigPath),
		gradeMock02Task8(ctx, kubeconfigPath),
		gradeMock02Task9(ctx, kubeconfigPath),
		gradeMock02Task10(ctx, kubeconfigPath),
		gradeMock02Task11(ctx, kubeconfigPath),
		gradeMock02Task12(ctx, kubeconfigPath),
	}}
}

func gradeMock02Task1(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	initImage := kubeEquals(ctx, kubeconfigPath, "busybox:1", "get", "pod", "bootstrap", "-n", "ops",
		"-o", `jsonpath={.spec.initContainers[?(@.name=="prep")].image}`)
	appImage := kubeEquals(ctx, kubeconfigPath, "nginx:alpine", "get", "pod", "bootstrap", "-n", "ops",
		"-o", `jsonpath={.spec.containers[?(@.name=="app")].image}`)
	volume := kubeEquals(ctx, kubeconfigPath, "{}", "get", "pod", "bootstrap", "-n", "ops",
		"-o", `jsonpath={.spec.volumes[?(@.name=="work")].emptyDir}`)
	initMount := kubeEquals(ctx, kubeconfigPath, "/work", "get", "pod", "bootstrap", "-n", "ops",
		"-o", `jsonpath={.spec.initContainers[?(@.name=="prep")].volumeMounts[?(@.name=="work")].mountPath}`)
	appMount := kubeEquals(ctx, kubeconfigPath, "/work", "get", "pod", "bootstrap", "-n", "ops",
		"-o", `jsonpath={.spec.containers[?(@.name=="app")].volumeMounts[?(@.name=="work")].mountPath}`)
	return examTask(1, "Init-container Pod", 8,
		examCheck("prep init container uses busybox:1", 2, initImage),
		examCheck("app container uses nginx:alpine", 2, appImage),
		examCheck("both containers mount emptyDir work at /work", 4, volume && initMount && appMount))
}

func gradeMock02Task2(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "nginx:alpine", "get", "deployment", "billing-api",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	replicas := kubeEquals(ctx, kubeconfigPath, "3", "get", "deployment", "billing-api",
		"-o", "jsonpath={.spec.replicas}")
	label := kubeEquals(ctx, kubeconfigPath, "backend", "get", "deployment", "billing-api",
		"-o", "jsonpath={.metadata.labels.tier}")
	return examTask(2, "Billing Deployment", 7,
		examCheck("Deployment uses nginx:alpine", 2, image),
		examCheck("Deployment requests 3 replicas", 3, replicas),
		examCheck("Deployment has label tier=backend", 2, label))
}

func gradeMock02Task3(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	ready := kubeEquals(ctx, kubeconfigPath, "True", "get", "pod", "checkout",
		"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
	return examTask(3, "Checkout application", 12,
		examCheck("checkout Pod is Ready", 12, ready))
}

func gradeMock02Task4(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	schedule := kubeEquals(ctx, kubeconfigPath, "*/5 * * * *", "get", "cronjob", "nightly-report",
		"-n", "ops", "-o", "jsonpath={.spec.schedule}")
	image := kubeEquals(ctx, kubeconfigPath, "busybox:1", "get", "cronjob", "nightly-report",
		"-n", "ops", "-o", "jsonpath={.spec.jobTemplate.spec.template.spec.containers[0].image}")
	suspend := kubeEquals(ctx, kubeconfigPath, "false", "get", "cronjob", "nightly-report",
		"-n", "ops", "-o", "jsonpath={.spec.suspend}")
	if kubeEquals(ctx, kubeconfigPath, "", "get", "cronjob", "nightly-report",
		"-n", "ops", "-o", "jsonpath={.spec.suspend}") {
		suspend = true
	}
	return examTask(4, "Nightly CronJob", 8,
		examCheck("schedule is */5 * * * *", 3, schedule),
		examCheck("job uses busybox:1", 3, image),
		examCheck("CronJob is not suspended", 2, suspend))
}

func gradeMock02Task5(ctx context.Context, node string) ExamTaskResult {
	output, err := dockerExec(ctx, node, "cat", "/root/gateway-crds.txt")
	got := strings.Fields(output)
	sort.Strings(got)
	want := []string{
		"gatewayclasses.gateway.networking.k8s.io",
		"gateways.gateway.networking.k8s.io",
		"httproutes.gateway.networking.k8s.io",
	}
	return examTask(5, "Gateway CRD inventory file", 6,
		examCheck("/root/gateway-crds.txt contains the Gateway API CRDs", 6,
			err == nil && strings.Join(got, "\n") == strings.Join(want, "\n")))
}

func gradeMock02Task6(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	color := kubeEquals(ctx, kubeconfigPath, "blue", "get", "widget", "storefront", "-n", "ops",
		"-o", "jsonpath={.spec.color}")
	replicas := kubeEquals(ctx, kubeconfigPath, "2", "get", "widget", "storefront", "-n", "ops",
		"-o", "jsonpath={.spec.replicas}")
	return examTask(6, "Widget custom resource", 8,
		examCheck("storefront color is blue", 4, color),
		examCheck("storefront replicas is 2", 4, replicas))
}

func gradeMock02Task7(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	target := kubeEquals(ctx, kubeconfigPath, "apps/v1 Deployment checkout-app 1 5", "get", "hpa", "checkout-hpa",
		"-o", "jsonpath={.spec.scaleTargetRef.apiVersion} {.spec.scaleTargetRef.kind} {.spec.scaleTargetRef.name} {.spec.minReplicas} {.spec.maxReplicas}")
	memory := kubeEquals(ctx, kubeconfigPath, "Resource memory Utilization 70", "get", "hpa", "checkout-hpa",
		"-o", "jsonpath={.spec.metrics[0].type} {.spec.metrics[0].resource.name} {.spec.metrics[0].resource.target.type} {.spec.metrics[0].resource.target.averageUtilization}")
	window := kubeEquals(ctx, kubeconfigPath, "60", "get", "hpa", "checkout-hpa",
		"-o", "jsonpath={.spec.behavior.scaleUp.stabilizationWindowSeconds}")
	return examTask(7, "Checkout Horizontal Pod Autoscaler", 10,
		examCheck("HPA targets checkout-app with bounds 1–5", 3, target),
		examCheck("memory utilization target is 70%", 4, memory),
		examCheck("scale-up stabilization is 60 seconds", 3, window))
}

func gradeMock02Task8(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	target := kubeEquals(ctx, kubeconfigPath, "apps/v1 Deployment billing-api", "get",
		"verticalpodautoscaler", "billing-vpa", "-o",
		"jsonpath={.spec.targetRef.apiVersion} {.spec.targetRef.kind} {.spec.targetRef.name}")
	mode := kubeEquals(ctx, kubeconfigPath, "Auto", "get", "verticalpodautoscaler", "billing-vpa",
		"-o", "jsonpath={.spec.updatePolicy.updateMode}")
	return examTask(8, "Billing Vertical Pod Autoscaler", 9,
		examCheck("VPA targets billing-api", 4, target),
		examCheck("update mode is Auto", 5, mode))
}

func gradeMock02Task9(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	class := kubeEquals(ctx, kubeconfigPath, "nginx", "get", "gateway", "shop-gateway",
		"-n", "edge", "-o", "jsonpath={.spec.gatewayClassName}")
	listener := kubeEquals(ctx, kubeconfigPath, "https HTTPS 443", "get", "gateway", "shop-gateway",
		"-n", "edge", "-o", "jsonpath={.spec.listeners[0].name} {.spec.listeners[0].protocol} {.spec.listeners[0].port}")
	return examTask(9, "Shop Gateway", 8,
		examCheck("Gateway uses class nginx", 3, class),
		examCheck("https listener uses HTTPS on port 443", 5, listener))
}

func gradeMock02Task10(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	parent := kubeEquals(ctx, kubeconfigPath, "shop-gateway", "get", "httproute", "shop-route",
		"-n", "edge", "-o", "jsonpath={.spec.parentRefs[0].name}")
	path := kubeEquals(ctx, kubeconfigPath, "/", "get", "httproute", "shop-route",
		"-n", "edge", "-o", "jsonpath={.spec.rules[0].matches[0].path.value}")
	backend := kubeEquals(ctx, kubeconfigPath, "shop", "get", "httproute", "shop-route",
		"-n", "edge", "-o", "jsonpath={.spec.rules[0].backendRefs[0].name}")
	return examTask(10, "Shop HTTPRoute", 6,
		examCheck("HTTPRoute attaches to shop-gateway", 3, parent),
		examCheck("path / is routed to Service shop", 3, path && backend))
}

func gradeMock02Task11(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "busybox:1", "get", "job", "import-job",
		"-n", "finance", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
	completions := kubeEquals(ctx, kubeconfigPath, "1", "get", "job", "import-job",
		"-n", "finance", "-o", "jsonpath={.spec.completions}")
	backoff := kubeEquals(ctx, kubeconfigPath, "2", "get", "job", "import-job",
		"-n", "finance", "-o", "jsonpath={.spec.backoffLimit}")
	complete := kubeEquals(ctx, kubeconfigPath, "True", "get", "job", "import-job",
		"-n", "finance", "-o", `jsonpath={.status.conditions[?(@.type=="Complete")].status}`)
	return examTask(11, "Import Job", 10,
		examCheck("Job uses busybox:1 with completions 1 and backoffLimit 2", 4, image && completions && backoff),
		examCheck("Job has completed", 6, complete))
}

func gradeMock02Task12(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	replicas := kubeEquals(ctx, kubeconfigPath, "4", "get", "deployment", "catalog",
		"-o", "jsonpath={.spec.replicas}")
	label := kubeEquals(ctx, kubeconfigPath, "prod", "get", "deployment", "catalog",
		"-o", "jsonpath={.metadata.labels.env}")
	return examTask(12, "Catalog scale and labels", 8,
		examCheck("catalog has 4 replicas", 4, replicas),
		examCheck("catalog Deployment has env=prod", 4, label))
}
