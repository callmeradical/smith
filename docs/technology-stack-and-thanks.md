# Technology Stack and Thanks

This page captures the core technologies Smith currently depends on, and acknowledges the projects that inspired its design direction.

## Core Runtime and Control Plane Technologies

- Go (`go 1.25.0`) for core services and CLI (`smith-api`, `smith-core`, `smith-replica`, `smithctl`).
- etcd (`go.etcd.io/etcd/client/v3`) as the authoritative store for loop state, locks, journals, handoffs, overrides, and audit records.
- Kubernetes (`k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`) as the execution substrate for control-plane deployments and replica Jobs.
- Helm (`helm/smith`) for packaging and deploying Smith components.
- Docker (`docker/*.Dockerfile`) for container images.

## Local Development and Environment Tooling

- vCluster for local multi-cluster-style integration workflows.
- k3d for local Kubernetes cluster provisioning.
- `make` + shell scripts (`scripts/integration`, `scripts/test`) for repeatable developer workflows and test orchestration.

## Interfaces and Operator Surfaces

- HTTP/JSON API (`cmd/smith-api`) for loop ingress, control, auth lifecycle, and reporting.
- CLI (`cmd/smithctl`) for operator automation and scripting.
- Console web shell (`console/`) for operator-facing runtime configuration and UI surface.

## Testing and Verification Tooling

- Go test for unit/integration coverage.
- Godog (`github.com/cucumber/godog`) for BDD acceptance scenarios.
- Testify (`github.com/stretchr/testify`) for assertions and test helpers.

## Thanks and Inspiration

Smith is built with direct reliance on and inspiration from the following ecosystems and projects:

- Kubernetes community and maintainers.
- Helm community and maintainers.
- vCluster maintainers.
- etcd maintainers.
- Go language and tooling maintainers.
- Docker maintainers.

Design and product inspiration:

- [marcus/td](https://github.com/marcus/td)
- [marcus/sidecar](https://github.com/marcus/sidecar)
- [Ralph](https://github.com/snarktank/ralph)

Thank you to all of these projects and communities for the ideas, infrastructure, and tooling that make Smith possible.
