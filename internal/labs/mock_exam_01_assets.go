package labs

const mockExamVPAAndGatewayCRDs = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: verticalpodautoscalers.autoscaling.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: autoscaling.k8s.io
  scope: Namespaced
  names:
    plural: verticalpodautoscalers
    singular: verticalpodautoscaler
    kind: VerticalPodAutoscaler
    shortNames: [vpa]
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            x-kubernetes-preserve-unknown-fields: true
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: verticalpodautoscalercheckpoints.autoscaling.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: autoscaling.k8s.io
  scope: Namespaced
  names:
    plural: verticalpodautoscalercheckpoints
    singular: verticalpodautoscalercheckpoint
    kind: VerticalPodAutoscalerCheckpoint
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gatewayclasses.gateway.networking.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: gateway.networking.k8s.io
  scope: Cluster
  names:
    plural: gatewayclasses
    singular: gatewayclass
    kind: GatewayClass
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              controllerName:
                type: string
            required: [controllerName]
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gateways.gateway.networking.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: gateway.networking.k8s.io
  scope: Namespaced
  names:
    plural: gateways
    singular: gateway
    kind: Gateway
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required: [gatewayClassName, listeners]
            properties:
              gatewayClassName:
                type: string
              listeners:
                type: array
                x-kubernetes-list-type: atomic
                items:
                  type: object
                  required: [name, protocol, port]
                  properties:
                    name:
                      type: string
                    protocol:
                      type: string
                    port:
                      type: integer
                      format: int32
`

const mockExamBaseResources = `apiVersion: v1
kind: Namespace
metadata:
  name: mc-namespace
---
apiVersion: v1
kind: Namespace
metadata:
  name: nginx-gateway
---
apiVersion: v1
kind: Namespace
metadata:
  name: kk-ns
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: nginx
spec:
  controllerName: exam.cka.local/nginx
---
apiVersion: v1
kind: Pod
metadata:
  name: messaging
  namespace: default
  labels:
    app: messaging
spec:
  containers:
  - name: messaging
    image: redis:7-alpine
    ports:
    - containerPort: 6379
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkapp-deploy
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kkapp
  template:
    metadata:
      labels:
        app: kkapp
    spec:
      containers:
      - name: webapp
        image: nginx:alpine
        resources:
          requests:
            cpu: 20m
            memory: 16Mi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: analytics-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: analytics
  template:
    metadata:
      labels:
        app: analytics
    spec:
      containers:
      - name: analytics
        image: nginx:alpine
        resources:
          requests:
            cpu: 20m
            memory: 16Mi
---
apiVersion: v1
kind: Pod
metadata:
  name: orange
  namespace: default
  labels:
    app: orange
spec:
  initContainers:
  - name: warmup
    image: busybox:1
    command: ["sh", "-c", "sleep 3600"]
  containers:
  - name: orange
    image: nginx:alpine
`

const mockExamHPASkeleton = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: webapp-hpa
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: kkapp-deploy
  minReplicas: 2
  maxReplicas: 10
`

const mockExamChartTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-podinfo
  labels:
    app.kubernetes.io/instance: {{ .Release.Name }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: {{ .Release.Name }}
    spec:
      containers:
      - name: podinfo
        image: {{ .Values.image }}
`
