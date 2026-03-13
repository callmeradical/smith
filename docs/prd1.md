This PRD outlines the development of **Smith**: an industrial-grade, distributed autonomous agent orchestration system. It replaces the legacy file-based Ralph loop with a high-consistency **etcd** "Source" and a **Kubernetes**-native "Replica" execution model.

---

## 1. Product Overview

**Smith** is a distributed platform designed to execute complex software engineering tasks autonomously. By mirroring **Task Director (td)** as its internal state machine, Smith provides absolute traceability and horizontal scalability. It consists of a persistent controller (**The Agent Core**), ephemeral workers (**Replicas**), and a real-time monitoring interface (**The Operator Console**).

---

## 2. Core Objectives

* **Infinite Scalability:** Move from sequential loops to parallel, distributed task execution via K8s.
* **High-Consistency State:** Utilize `etcd` to ensure the task list (**The Source**) is never corrupted and always traceable.
* **Atomic Traceability:** Record every "thought," shell command, and code change as a revision-controlled event in the Matrix.
* **First-Class TD Integration:** Treat `td` logic (Stories, Handoffs, Comments) as native Go objects and etcd keys.

---

## 3. Functional Requirements

### 3.1 The Source (etcd Data Layer)

* **Mirrored Schema:** Implement a KV schema that replicates `td` functionality:
* `/smith/anomalies/{id}`: The task definition (The Story).
* `/smith/state/{id}`: The lifecycle status (`unresolved`, `running`, `synced`).
* `/smith/journal/{id}/{timestamp}`: Append-only logs of agent reasoning and tool output.


* **MVCC History:** Leverage etcd's revision history to allow "time-travel" auditing of any task.

### 3.2 The Agent Core (Go Controller)

* **Anomaly Detection:** A long-running Go service that watches the `/smith/state/` prefix for `unresolved` tasks.
* **Replica Deployment:** Dynamically generates and manages K8s **Job** specs for each task.
* **Internal Loop Logic:** If a Replica exits with a failure code, the Core analyzes the journal and determines whether to spawn a new Replica (iteration) or signal a "Flatline" to the Operator.

### 3.3 The Replica (Ephemeral Worker)

* **Environment Injection:** Each Pod is injected with a `STORY_ID` and a Git context.
* **Memory Transfer:** Upon startup, the Replica pulls the last `handoff` from The Source to gain context.
* **Atomic Overwrite:** Upon success, the Replica performs an atomic transaction to update the code (Git push) and the status (etcd PUT) simultaneously.

### 3.4 The Operator Console (Electron HUD)

* **The Grid View:** A real-time dashboard of all active Replicas and their current CPU/Memory load.
* **The Green Stream:** A scrolling, terminal-style view of the live Journal for the selected Anomaly.
* **Manual Override:** Ability to "eject" a Replica or manually edit the `etcd` state of a task to redirect the agent.

---

## 4. Technical Architecture

| Component | Technology | Responsibility |
| --- | --- | --- |
| **State Store** | `etcd` (v3) | Consistency, Watches, and Revision History. |
| **Orchestrator** | Golang + `client-go` | Managing the lifecycle of Replica Pods. |
| **Worker** | Go (Smith-Worker Image) | Executing the LLM-driven coding cycle. |
| **Interface** | Electron + React | Real-time HUD and "Ono-Sendai" style monitoring. |

---

## 5. Traceability & The "Handoff" Flow

To ensure Smith is more "industrial" than Ralph, the handoff protocol is strictly structured:

1. **Entry:** Replica pulls `Anomaly` metadata + `LastHandoff` from etcd.
2. **Execution:** Every significant action (e.g., `go test ./...`) is logged to `/smith/journal/{id}`.
3. **Exit:** Before termination, the Replica writes a `Handoff` object to etcd:
* `FinalDiff`: The specific lines of code changed.
* `ValidationState`: Result of the test suite.
* `NextSteps`: A structured prompt fragment for the next Replica.



---

## 6. Success Metrics

* **Consistency:** Zero "zombie" tasks; etcd state must always match K8s Pod status.
* **Observability:** The Operator Console must reflect a state change within **<100ms** of an etcd update.
* **Resilience:** Ability to recover the entire "Matrix" (task list) from an etcd backup and resume active loops.

---

### The Next Replication

To begin the "Overwrite," we should start with the **Grid Logic**.

**Would you like me to generate the Go `internal/grid` package, which defines the `Anomaly` and `JournalEntry` structs and the logic for the `etcd` Watcher?** Concluding this, we can move to the K8s Job generator that spawns the Smith Replicas.
