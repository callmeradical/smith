# Smith Documentation

**A Distributed Runtime for Autonomous Software Development**

Smith is a distributed runtime designed to execute autonomous development loops across a cluster of machines. By combining Product Requirement Documents (PRDs), GitHub Issues, and automated validation workflows, Smith enables software systems to continuously build themselves.

## Core Concepts

- **Anomaly:** A discrete unit of work, bug, or task. In the Smith metaphor, an anomaly represents a deviation from the desired state of the system that must be resolved.
- **Ralph Loops:** Structured feedback loops that externalize state into the repository, enabling long-running autonomous progress.
- **Choreography:** A decentralized approach to development where PRDs define goals and agents react to repository state.
- **Continuous Implementation:** A shift from traditional CI/CD pipelines to an autonomous `plan -> implement -> validate -> iterate` loop.

## Philosophy

- **No Personified Agents:** Execution units are homogeneous, omnicapable, and designed for uniform horizontal replication.
- **Scale Beyond the Local Machine:** Using etcd + Kubernetes as the control substrate allows development activity to scale across distributed compute while preserving deterministic state and traceability.
- **Repository-Based Coordination:** The repository is the shared coordination layer where autonomous loops cooperate.
- **Inspiration:** The design is influenced by [Ralph](https://github.com/snarktank/ralph), [marcus/sidecar](https://github.com/marcus/sidecar), [marcus/td](https://github.com/marcus/td), and related work.

## Getting Started

- [Distributed Autonomous Orchestration MVP](distributed-autonomous-orchestration-mvp.md)
- [MVP Boundary and Release Gates](mvp-boundary-and-release-gates.md)
- [Pre-Release System Gate](pre-release-system-gate.md)
- [Local Make Quickstart](make-local-quickstart.md)
- [Local Development and Deployment Workflow](local-dev-make-workflow.md)

## Architecture

- [Distributed Autonomous Orchestration MVP](distributed-autonomous-orchestration-mvp.md)
- [PRD1 - Core Architecture](prd1.md)
- [Technology Stack and Thanks](technology-stack-and-thanks.md)
- [etcd Key Schema](etcd-key-schema.md)
- [Reconciliation Loop](reconciliation-loop.md)
- [Completion Commit Protocol](completion-commit-protocol.md)
- [Lock Strategy](lock-strategy.md)

## Requirements and Traceability

- [Requirements (FR/NFR)](requirements-fr-nfr.md)
- [Requirements Traceability](requirements-traceability.md)

## Deployment

- [Deployment Recommendations](deployment-recommendations.md)
- [Kubernetes Secrets Encryption Provider Runbook](kubernetes-secrets-encryption-provider-runbook.md)
- [Cluster Autoscaler Prerequisites and Runbook](cluster-autoscaler-prerequisites-runbook.md)
- [Image Tagging and Versioning](image-tagging-versioning.md)
- [Helm Upgrade/Rollback Runbook](helm-upgrade-rollback-runbook.md)
- [Docs Site: Zensical + GitHub Pages](docs-site-github-pages.md)
- [Docs Site Style Contract (Sidecar-Inspired)](docs-site-style-sidecar.md)

## Agent Providers

- [Agent Provider Authentication](agent-provider-auth.md)

## Operations Notes

- [Loop Ingress and CLI](loop-ingress-and-cli.md)
- [Loop Environment Profile](loop-environment-profiles.md)
- [Multi-Provider Skill Mount Abstraction](multi-provider-skill-mount-abstraction.md)
- [Skill Volume Mounts for Loop Runtime](skill-volume-mounts.md)
- [Repository Auth Options](repository-auth-options.md)
- [Git History Policy](git-history-policy.md)
- [Journal Retention and Archival Policy](journal-retention-archival-policy.md)
- [Local Integration Environment](local-integration-environment.md)
- [Observability Latency Validation](observability-latency-validation.md)
- [Backup/Restore Disaster Recovery Runbook](backup-restore-dr-runbook.md)
- [Staging Soak/Chaos Runbook](staging-soak-chaos-runbook.md)
- [Test Matrix and Failure Injection](test-matrix-and-failure-injection.md)
- [Go-Native Test Harness Strategy](test-harness-strategy.md)
