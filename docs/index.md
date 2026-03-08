# Smith Documentation

Smith is an etcd-backed, Kubernetes-native autonomous orchestration platform.

## Philosophy

- Smith intentionally avoids anthropomorphized agent personas.
- Workers are homogeneous and omnicapable, designed for uniform replication.
- The platform extends beyond a local file-system execution model by using etcd + Kubernetes for distributed orchestration and traceable state.
- The design is influenced by Ralph, `marcus/sidecar`, `marcus/td`, and related work, while targeting scale beyond a single developer machine.

## Getting Started

- [Distributed Autonomous Orchestration MVP](distributed-autonomous-orchestration-mvp.md)
- [MVP Boundary and Release Gates](mvp-boundary-and-release-gates.md)
- [Pre-Release System Gate](pre-release-system-gate.md)
- [Local Make Quickstart](make-local-quickstart.md)

## Architecture

- [Distributed Autonomous Orchestration MVP](distributed-autonomous-orchestration-mvp.md)
- [etcd Key Schema](etcd-key-schema.md)
- [Reconciliation Loop](reconciliation-loop.md)
- [Completion Commit Protocol](completion-commit-protocol.md)
- [Lock Strategy](lock-strategy.md)

## Requirements and Traceability

- [Requirements (FR/NFR)](requirements-fr-nfr.md)
- [Requirements Traceability](requirements-traceability.md)

## Deployment

- [Deployment Recommendations](deployment-recommendations.md)
- [Cluster Autoscaler Prerequisites and Runbook](cluster-autoscaler-prerequisites-runbook.md)
- [Image Tagging and Versioning](image-tagging-versioning.md)
- [Helm Upgrade/Rollback Runbook](helm-upgrade-rollback-runbook.md)
- [Docs Site: Zensical + GitHub Pages](docs-site-github-pages.md)

## Agent Providers

- [Agent Provider Authentication](agent-provider-auth.md)

## Operations Notes

- [Loop Ingress and CLI](loop-ingress-and-cli.md)
- [Loop Environment Profile](loop-environment-profile.md)
- [Multi-Provider Skill Mount Abstraction](multi-provider-skill-mount-abstraction.md)
- [Repository Auth Options](repository-auth-options.md)
- [Git History Policy](git-history-policy.md)
- [Journal Retention and Archival Policy](journal-retention-archival-policy.md)
- [Local Integration Environment](local-integration-environment.md)
- [Observability Latency Validation](observability-latency-validation.md)
- [Backup/Restore Disaster Recovery Runbook](backup-restore-dr-runbook.md)
- [Staging Soak/Chaos Runbook](staging-soak-chaos-runbook.md)
- [Test Matrix and Failure Injection](test-matrix-and-failure-injection.md)
- [Go-Native Test Harness Strategy](test-harness-strategy.md)
