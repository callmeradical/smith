# Initial Concept
Smith is an etcd-backed, Kubernetes-native autonomous orchestration platform.

# Product Definition

## Vision
To provide a robust, distributed, and deterministic substrate for autonomous agent execution loops, moving beyond single-machine constraints.

## Target Audience
- Platform engineers building autonomous agent workflows.
- SRE/DevOps teams requiring high-scale, traceable execution.
- Developers of agentic systems needing Kubernetes-native orchestration.

## Core Value Propositions
- **Distributed State:** Uses etcd as the source of truth for execution state and concurrency control.
- **Scalability:** Leverages Kubernetes Jobs for horizontal scaling of execution units (replicas).
- **Traceability:** Provides a comprehensive journal, handoff, and audit trail for every execution loop.
- **Safety:** Implements per-loop locks and revision-checked state transitions to prevent race conditions.

## Key Features
- **Control Plane:** API and Core components for managing loop lifecycles.
- **Ingress Adapters:** Support for GitHub issues, PRDs, and direct operator requests.
- **Autonomous Dev Loop:** End-to-end flow from GitHub issue to PRD, technical planning, TDD implementation, and PR submission.
- **Operator CLI (smithctl):** A powerful tool for inspecting and managing loops.
- **Operator Console:** A web-based UI for real-time monitoring and terminal interaction.
- **Replica Workers:** Uniform, scalable execution units that perform the actual work.

## Success Metrics
- **Reliability:** Percentage of loops reaching a terminal state correctly.
- **Scale:** Number of concurrent loops handled by the control plane.
- **Observability:** Latency from request to replica start and total execution visibility.
