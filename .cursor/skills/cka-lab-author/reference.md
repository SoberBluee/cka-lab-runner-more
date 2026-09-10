# CKA Lab Author — Reference

## Verify invariants by domain

Use these as the *extra* checks beyond “symptom fixed.”

### Storage (PV/PVC)

| Intended fault | Must still be true after fix |
|----------------|------------------------------|
| Access mode mismatch | Same `storageClassName` on PV/PVC; same PV name if prompt says use existing PV; size still satisfies claim |
| Wrong `reclaimPolicy` | Capacity, accessModes, SC name unchanged unless prompt says otherwise |
| Wrong SC name on claim | Only `storageClassName` (and binding) change; do not require inventing a new SC unless Creation task |

### Networking / Services

| Intended fault | Must still be true |
|----------------|-------------------|
| Selector mismatch | Service name/port unchanged; only selector labels corrected |
| Wrong `targetPort` | Selector and Service port unchanged |
| NetworkPolicy peer/port | Deny-all / required policy still present if prompt said keep lockdown; only allow rule fixed |

### RBAC / identity

| Intended fault | Must still be true |
|----------------|-------------------|
| Missing verbs | Subject SA unchanged; no cluster-admin |
| Wrong SA on pod | Role rules unchanged if already correct |
| automount false | Role/RoleBinding unchanged; token path appears; `can-i` still yes |
| Creation: new Role/Binding | Exact verbs/resources/names from prompt; reject `*` unless asked |

### Scheduling

| Intended fault | Must still be true |
|----------------|-------------------|
| Missing toleration | Node taints remain; do not accept “remove the taint” |
| Wrong affinity | Taints remain if present |

### Control plane / node

| Intended fault | Must still be true |
|----------------|-------------------|
| Bad static pod image tag | Component back to API-server-matching tag; workload reconciles |
| kubelet / crictl host fix | Marker files / unit state as defined; node Ready |

## Artifact verification

When the prompt requires a file under e.g. `/opt/CKA/`:

1. In kind labs, create the directory in `Break`/`Prepare` on the control-plane node (or document that the candidate creates it).
2. In `Verify`, `dockerExec` / read the file and check contents (exact string, JSONPath output, or revision number).
3. Fail if the file is missing or content does not match.

## Prompt examples (task types)

### Troubleshooting

```text
[Weight: 6%] | Time limit: 5–7 minutes
Context: Pod postgres in finance is Pending. Database never becomes available.
Task: Get pod postgres in namespace finance to Running. Use the existing PersistentVolume.
Constraints: Do not change the PersistentVolume name or storageClassName.
```

### Creation

```text
[Weight: 5%] | Time limit: 6–8 minutes
Context: A new developer joined team apps.
Task: In namespace apps, create ServiceAccount dev, Role, and RoleBinding so the SA can create, list, and watch pods and deployments. Write kubectl auth can-i create deployments -n apps --as=system:serviceaccount:apps:dev output to /opt/CKA/rbac-check.txt
Constraints: Least privilege; do not grant delete or secrets access.
```

### Mix

```text
[Weight: 8%] | Time limit: 7–10 minutes
Context: api Deployment in shop cannot reach cache. A cache Service is missing.
Task: Fix connectivity so the api pods can reach Service cache on port 6379. Create any missing Service resources required by name cache. Remove any temporary debug pods when done.
```

## Docs search terms (SolutionSteps Notes)

| Topic | Search on kubernetes.io/docs |
|-------|---------------------------|
| NetworkPolicy | `NetworkPolicy` |
| Ingress | `Ingress` |
| PVC/PV | `PersistentVolumeClaim` |
| RBAC | `Using RBAC Authorization` |
| Service | `Service` |
| DaemonSet | `DaemonSet` |
| Pod scheduling taints | `Taints and Tolerations` |
| Static pods | `Static Pods` |
| kubeadm upgrade | `kubeadm upgrade` |
| etcd snapshot | `etcd` `backup` `restore` |

## Local vs exam node access

| Exam prompt | kind/lab runner equivalent |
|-------------|----------------------------|
| `ssh <node>` then `sudo -i` | `docker exec -it <node-container> bash` |
| `journalctl -u kubelet` | same inside node container if systemd present; else check kubelet logs/markers per lab |
| `crictl ps` | available on kind nodes |
| Write `/opt/CKA/...` | ensure path exists on the node or use a mounted path the Verify can read |

Put exam wording in `Description()`. Put the kind mapping only in `SolutionSteps` Notes.
