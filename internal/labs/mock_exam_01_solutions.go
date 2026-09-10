package labs

func mockExam01Solutions() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Task 1 — create the multi-container Pod",
			Command: `kubectl create namespace mc-namespace
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: mc-pod
  namespace: mc-namespace
spec:
  containers:
  - name: mc-pod-1
    image: nginx:1-alpine
    env:
    - name: NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
  - name: mc-pod-2
    image: busybox:1
    command: ["sh", "-c", "while true; do date >> /var/log/shared/date.log; sleep 1; done"]
    volumeMounts:
    - name: shared-volume
      mountPath: /var/log/shared
  - name: mc-pod-3
    image: busybox:1
    command: ["sh", "-c", "touch /var/log/shared/date.log; tail -f /var/log/shared/date.log"]
    volumeMounts:
    - name: shared-volume
      mountPath: /var/log/shared
  volumes:
  - name: shared-volume
    emptyDir: {}
EOF`,
			Notes: "emptyDir survives a container restart but is deleted when the Pod is removed from its node.",
		},
		{
			Description: "Task 2 — repair the CRI client configuration",
			Command: `docker exec -it cka-lab-control-plane sh -c 'cat >/etc/crictl.yaml <<EOF
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 2
debug: false
EOF
crictl info'`,
		},
		{
			Description: "Task 3 — save the VPA CRD names on the node",
			Command:     `docker exec cka-lab-control-plane sh -c "kubectl get crd -o name | cut -d/ -f2 | grep verticalpodautoscaler | sort > /root/vpa-crds.txt"`,
		},
		{
			Description: "Task 4 — expose the messaging Pod imperatively",
			Command:     "kubectl expose pod messaging --name=messaging-service --port=6379",
		},
		{
			Description: "Task 5 — create the HR Deployment",
			Command:     "kubectl create deployment hr-web-app --image=kodekloud/webapp-color --replicas=2",
		},
		{
			Description: "Task 6 — shorten the blocking orange init container",
			Command: `kubectl get pod orange -o yaml > /tmp/orange.yaml
# Remove status and generated metadata, then change the warmup command from sleep 3600 to sleep 2.
kubectl delete pod orange
kubectl apply -f /tmp/orange.yaml`,
			Notes: "Pod container commands are immutable, so the corrected Pod must be recreated.",
		},
		{
			Description: "Task 7 — create the NodePort Service",
			Command: `kubectl expose deployment hr-web-app --name=hr-web-app-service --type=NodePort --port=8080 --target-port=8080
kubectl patch service hr-web-app-service -p '{"spec":{"ports":[{"port":8080,"targetPort":8080,"nodePort":30082}]}}'`,
		},
		{
			Description: "Task 8 — create the PersistentVolume",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-analytics
spec:
  capacity:
    storage: 100Mi
  accessModes:
  - ReadWriteMany
  hostPath:
    path: /pv/data-analytics
EOF`,
		},
		{
			Description: "Task 9 — complete and create the HPA",
			Command: `docker exec -it cka-lab-control-plane bash
cat >>/root/webapp-hpa.yaml <<'EOF'
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
EOF
kubectl apply -f /root/webapp-hpa.yaml
exit`,
		},
		{
			Description: "Task 10 — create the VPA",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: analytics-vpa
  namespace: default
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: analytics-deployment
  updatePolicy:
    updateMode: Recreate
EOF`,
		},
		{
			Description: "Task 11 — create the Gateway",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: web-gateway
  namespace: nginx-gateway
spec:
  gatewayClassName: nginx
  listeners:
  - name: http
    protocol: HTTP
    port: 80
EOF`,
		},
		{
			Description: "Task 12 — refresh the repository and upgrade the release",
			Command: `helm repo update kk-mock1
helm search repo kk-mock1/podinfo --versions
helm upgrade kk-mock1 kk-mock1/podinfo -n kk-ns --version 6.11.2
helm list -n kk-ns`,
		},
	}
}
