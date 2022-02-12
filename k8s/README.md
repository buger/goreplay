# Native k8s integration

After following steps below, you will be able to capture traffic inside k8s like this:

```
gor --input-raw k8s://namespace/deployment/app:80 --output-http http://replay.com
```

GoReplay will running as a daemonset (e.g. on each phisical k8s node. 
It will also require giving required permission to have read access to K8s APIs, so it can dynamically filter traffic for a specific pods.

Supported format for filtering required pods:

```
k8s://[namespace/]pod/[pod_name] - k8s://default/pod/nginx-7848d4b86f-5nxz8
k8s://[namespace/]deployment/[deployment_name] - k8s://default/deployment/nginx
k8s://[namespace/]daemonset/[daemonset_name] - k8s://default/daemonset/nginx
k8s://[namespace/]labelSelector/[selector] - k8s://default/labelSelector/app=nginx
k8s://[namespace/]fieldSelector/[selector] - k8s://default/fieldSelector/metadata.name=nginx-7848d4b86f-5nxz8
```
`namespace` is optional, omit to use all namespaces: `k8s://labelSelector/app=replay`

## 1. Create a namespace
`kubectl create namespace goreplay`

## 2. Create the Kubernetes service account in the namespace:

`kubectl create serviceaccount goreplay --namespace goreplay`

## 3. Create Cluster Role which gives read-only access to the pods:
`kubectl -n goreplay -f clusterrole.yaml apply`

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: pod-reader
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
- apiGroups: [""]
  resources: ["deployments"]
  verbs: ["get", "watch", "list"]
- apiGroups: [""]
  resources: ["daemonset"]
  verbs: ["get", "watch", "list"]
```

## 4. Attach role to goreplay service account
`kubectl -n goreplay -f rolebinding.yaml apply`

```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: goreplay-reader-binding
subjects:
- kind: ServiceAccount
  name: goreplay
  namespace: goreplay
roleRef:
  kind: ClusterRole
  name: pod-reader
  apiGroup: ""
```

## 5. Start goreplay daemonset

`kubectl -n goreplay -f goreplay.yaml apply`

In arguments, specify which service you want to capture. 
Following format supported:


```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: goreplay-daemon
spec:
  template:
    spec:
      hostNetwork: true
      serviceAccountName: goreplay
      containers:
      - name: goreplay
        image: buger/goreplay:2.0.0-rc2
        command:
          - "--input-raw k8s://deployments/nginx:80"
          - "--output-stdout"
```

## 6. Create a simple http service (Optionally)

`kubectl -n default -f nginx.yaml apply`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
   
---
apiVersion: v1
kind: Service
metadata:
  name: ngnix-service
spec:
  selector:
    app: nginx
  type: NodePort
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80

```


## 7. Verify installation

Find url for your service using `kubectl get svc` or `minikube service --url ngnix-service -n http`, and make a call to it.

Get GoReplay logs, and check if it capture traffic of your service.
`kubectl logs -n goreplay -l name=goreplay-daemon --all-containers`

