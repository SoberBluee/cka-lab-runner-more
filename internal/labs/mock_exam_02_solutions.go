package labs

func mockExam02Solutions() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Task 1 — create the init-container Pod",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: bootstrap
  namespace: ops
spec:
  initContainers:
  - name: prep
    image: busybox:1
    command: ["sh", "-c", "echo ready > /work/ready"]
    volumeMounts:
    - name: work
      mountPath: /work
  containers:
  - name: app
    image: nginx:alpine
    volumeMounts:
    - name: work
      mountPath: /work
  volumes:
  - name: work
    emptyDir: {}
EOF`,
		},
		{
			Description: "Task 2 — create the billing Deployment",
			Command: `kubectl create deployment billing-api --image=nginx:alpine --replicas=3
kubectl label deployment billing-api tier=backend`,
		},
		{
			Description: "Task 3 — fix checkout by providing the missing ConfigMap",
			Command: `kubectl create configmap checkout-config --from-literal=app.conf='ok'
kubectl get pod checkout -o yaml > /tmp/checkout.yaml
# Point the volume at checkout-config, strip status, then recreate the Pod.
kubectl delete pod checkout
kubectl apply -f /tmp/checkout.yaml`,
			Notes: "The volume references checkout-config-missing. Creating a ConfigMap with that name also works.",
		},
		{
			Description: "Task 4 — create the CronJob",
			Command:     `kubectl create cronjob nightly-report -n ops --image=busybox:1 --schedule='*/5 * * * *' -- echo done`,
		},
		{
			Description: "Task 5 — save Gateway API CRD names on the node",
			Command:     `docker exec cka-lab-control-plane sh -c "kubectl get crd -o name | cut -d/ -f2 | grep gateway.networking.k8s.io | sort > /root/gateway-crds.txt"`,
		},
		{
			Description: "Task 6 — create the Widget",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: ops.exam.local/v1
kind: Widget
metadata:
  name: storefront
  namespace: ops
spec:
  color: blue
  replicas: 2
EOF`,
			Notes: "kubectl explain widget.spec lists the required fields.",
		},
		{
			Description: "Task 7 — create the memory HPA",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: checkout-hpa
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: checkout-app
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 70
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
EOF`,
		},
		{
			Description: "Task 8 — create the VPA in Auto mode",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: billing-vpa
  namespace: default
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: billing-api
  updatePolicy:
    updateMode: Auto
EOF`,
		},
		{
			Description: "Task 9 — create the HTTPS Gateway",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: shop-gateway
  namespace: edge
spec:
  gatewayClassName: nginx
  listeners:
  - name: https
    protocol: HTTPS
    port: 443
EOF`,
		},
		{
			Description: "Task 10 — create the HTTPRoute",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: shop-route
  namespace: edge
spec:
  parentRefs:
  - name: shop-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: shop
      port: 80
EOF`,
		},
		{
			Description: "Task 11 — create the Job",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: import-job
  namespace: finance
spec:
  completions: 1
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: import
        image: busybox:1
        command: ["sh", "-c", "echo imported"]
EOF`,
		},
		{
			Description: "Task 12 — scale and label catalog",
			Command: `kubectl scale deployment catalog --replicas=4
kubectl label deployment catalog env=prod`,
		},
	}
}
