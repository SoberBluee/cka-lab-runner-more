---
name: cka-lab-author
description: >-
  Author CKA practice labs for cka-lab-runner (Go Lab interface in internal/labs/).
  Use when creating, adding, editing, or designing labs, scenarios, Break/Verify
  logic, exam-style prompts, or lab_*.go files in this repository.
---

# CKA Lab Author

When creating or editing labs in this repo, follow the **Lab Authoring Rules** below exactly. Implement each lab as a Go type in `internal/labs/lab_<id>.go` that registers via `init()` and satisfies `Lab`.

## Lab Authoring Rules

- **No Hints:** Do not include any hints, shortcuts, or solution snippets in the prompt body that allow the user to complete the lab faster.
- **Exam-Style Formatting:** Format all questions to directly mimic real exam tasks, complete with context setups, tasks, and constraints.
- **Weight & Time Constraints:** Every scenario must include an exam-realistic point weight (e.g., [Weight: 7%]) and a strict time limit (e.g., 5–8 minutes) to simulate exam pressure.
- **Minimal Mutation Verification:** Strictly check only the target configuration that requires fixing. Modifying extraneous parameters (e.g., changing a StorageClass when only accessModes or persistentVolumeReclaimPolicy need fixing) is strictly evaluated as a fail.
- **Category Rotation:** when creating a lab, you will create them from these 3 categories. 1. Troubleshooting 2. Creation (a question may require you to create a new role and role binding from scratch because the question says something like 'a new developer has been added to the team and needs the ability to create, list and watch pods and deployments', or 'the development team needs a new caching application for the api, implement redis with a service and link it to the api pod'), 3. mix of both troubleshooting and creation
- **Explicit Context & Namespace:** Every lab must begin with a `kubectl config use-context <cluster-name>` instruction and mandate a specific target namespace. Resources placed in the wrong context or namespace are graded as 0.
- **Exact File Paths & Artifacts:** Questions must frequently require outputting CLI results, JSONPath filters, logs, or backups to specific file paths (e.g., `/opt/CKA/output.txt`).
- **Strict Naming & Clean Environment:** Resource names, labels, and specs must match exact casing. Any temporary debugging resources or leftover test pods created during task execution must be removed upon completion.
- **Node Hopping & OS-Level Troubleshooting:** Require the user to occasionally `ssh` into specific worker or control-plane nodes and escalate privileges (`sudo -i`) to fix host-level issues like `kubelet` configuration, static pod manifests, or container runtimes using `crictl` or `journalctl`.
- **Documentation-First Solutions:** When providing the final solution, emphasize imperative commands. For declarative YAML needs, provide the exact search terms to find the snippet on `kubernetes.io/docs` instead of just providing the finished YAML block.

## Map Rules → Go Lab Interface

| Method | Requirement |
|--------|-------------|
| `ID()` | `snake_case`, unique; symptom-oriented, not root-cause spoilers (e.g. `postgres_pending`, not `pvc_access_mode_wrong`) |
| `Title()` | Short exam-ticket style title; no spoilers |
| `Category()` | Use existing enum: `control-plane`, `networking`, `scheduling`, `dns`, `storage`, `workloads`, `rbac`, `security` — pick by **symptom domain**, not task type |
| `Difficulty()` | `easy` / `medium` / `hard` aligned to weight/time |
| `EstimatedTime()` | Upper bound of the stated time limit (minutes), as an `int` |
| `Description()` | Full exam prompt only (see template). No root cause, no fix steps |
| `Hints()` | **Always** `return nil` or `return []string{}` — never hint content |
| `Tags()` | Include task type: `troubleshooting`, `creation`, or `mixed`; plus topic tags; never spoil the fault |
| `Prepare` / `Break` | Context setup only. Creation labs: incomplete baseline. Troubleshooting: inject the fault. Mix: both |
| `VerifyBroken` | Assert broken/incomplete baseline before the user starts |
| `Verify` | **Minimal mutation** only — see below |
| `SolutionSteps()` | Imperative-first; docs search terms for YAML — see below |

Also update `README.md` lab list when adding a lab. Extend `cleanup.go` if you introduce new namespaces, CRDs, taints, cluster-scoped RBAC, or node labels.

## Description Template

Use this structure inside `Description()` (adapt names; keep sections):

```text
Set the context and namespace before doing any work:

  kubectl config use-context <cluster-name>
  # All work for this task must be done in namespace <namespace>

[Weight: N%] | Time limit: M–K minutes

Context:
<1–3 sentences of business/cluster context. Symptoms only. No root cause.>

Task:
1. <required action or end state>
2. <optional artifact path requirement>
3. <optional cleanup of temporary resources>

Constraints:
- Work only in context <cluster-name> and namespace <namespace> (wrong placement = 0).
- Resource names, labels, and field values must match exactly as specified (case-sensitive).
- Do not leave temporary debug pods, Jobs, or test resources behind.
- <any additional hard constraints: keep existing SC name, do not delete X, etc.>
```

For labs that need artifacts, include explicit paths, e.g. write output to `/opt/CKA/output.txt`.

For node/OS labs, the Context/Task must state the node name and that the candidate should use `ssh` / `sudo -i` (in kind-based local runs, solution notes may map this to `docker exec` on the node container — the **prompt** still uses exam wording).

## Category Rotation (Task Type)

When the user asks for a new lab (or a batch), choose and rotate among:

1. **Troubleshooting** — something is broken; fix to a stated end state.
2. **Creation** — build new resources from requirements (RBAC, Deployments/Services, NetPol, etc.).
3. **Mix** — diagnose a failure **and** create missing pieces.

State the chosen task type in `Tags()` (`troubleshooting` | `creation` | `mixed`). Prefer rotating so consecutive new labs are not the same task type unless the user asks otherwise.

## Minimal Mutation Verification

`Verify` must fail if the user “fixed” the symptom by changing unrelated fields.

**Do:**

- Assert the exact fields that define the intended fix (e.g. PVC `accessModes` == `ReadWriteOnce`, `storageClassName` still `manual`).
- Assert required names, namespaces, labels, and artifact file contents/paths when the prompt requires them.
- Assert end behaviour the prompt requires (pod Running, `auth can-i`, HTTP reachability) **in addition to** config constraints.
- For Creation labs: assert created objects exist with the exact names/verbs/selectors specified — reject over-broad grants when the prompt implies least privilege.

**Do not:**

- Accept any Bound PVC / Running pod regardless of how it was achieved.
- Allow deleting required objects and replacing with differently named substitutes unless the prompt allows it.
- Check unrelated cluster state.

**Pattern:** capture the invariant baseline in `Break`, then in `Verify` re-read those fields and compare.

```go
// Example invariant: SC name must remain "manual"; only accessModes may change
sc, _ := kubectl(ctx, kubeconfigPath, "get", "pvc", "postgres-data", "-n", "finance",
    "-o", "jsonpath={.spec.storageClassName}")
if strings.TrimSpace(sc) != "manual" {
    return fmt.Errorf("storageClassName must remain %q (got %q)", "manual", strings.TrimSpace(sc))
}
```

## SolutionSteps Rules

1. Prefer **imperative** commands (`kubectl create`, `kubectl patch`, `kubectl set`, `kubectl expose`, `kubectl run` with generators where still valid).
2. When YAML is required, **do not** dump a full spoilery manifest as the primary answer. Use Notes like:
   - Docs search: `kubernetes.io NetworkPolicy` / `ingress` / `persistentvolumeclaim`
   - Then minimal imperative apply or patch once the candidate would have adapted the docs snippet.
3. Include verification commands the candidate should re-run.
4. For kind/local node access, Notes may say: exam uses `ssh <node>` + `sudo -i`; this lab environment equivalent is `docker exec -it <node> bash`.
5. Never put solution content in `Description()` or `Hints()`.

## Implementation Checklist

Before finishing a new lab:

- [ ] `Hints()` empty; Description has no hints/spoilers
- [ ] Weight + time limit in Description; `EstimatedTime()` matches
- [ ] `use-context` + mandatory namespace in Description
- [ ] Task type is troubleshooting, creation, or mixed (rotated)
- [ ] `Verify` enforces minimal mutation / exact names
- [ ] Artifact paths verified when required
- [ ] Cleanup extended if needed; `go test ./internal/labs/` passes
- [ ] README lab list updated
- [ ] `SolutionSteps` imperative-first + docs search terms for YAML

## File Skeleton

```go
package labs

import (
	"context"
	"fmt"
)

func init() {
	Register(&ExampleLab{})
}

type ExampleLab struct {
	BaseLab
}

func (l *ExampleLab) ID() string             { return "example_lab" }
func (l *ExampleLab) Title() string          { return "Example Exam Task" }
func (l *ExampleLab) Category() Category     { return CategoryStorage }
func (l *ExampleLab) Difficulty() Difficulty { return DifficultyMedium }
func (l *ExampleLab) EstimatedTime() int     { return 8 }
func (l *ExampleLab) Tags() []string {
	return []string{"mixed", "pvc", "storage"}
}
func (l *ExampleLab) Hints() []string { return nil }

func (l *ExampleLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace finance

[Weight: 7%] | Time limit: 5–8 minutes

Context:
...

Task:
...

Constraints:
...`
}

func (l *ExampleLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ExampleLab) Break(ctx context.Context, kubeconfigPath string) error { /* ... */ return nil }
func (l *ExampleLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error { return nil }
func (l *ExampleLab) Verify(ctx context.Context, kubeconfigPath string) error { /* strict */ return nil }

func (l *ExampleLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{Description: "...", Command: "kubectl ...", Notes: "Docs search: kubernetes.io ..."},
	}
}
```

## Additional Detail

For Verify field matrices and prompt examples by task type, see [reference.md](reference.md).
