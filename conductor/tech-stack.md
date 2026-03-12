# Tech Stack

## Programming Languages
- **Go (1.25.0):** Used for all backend services, CLI, and core logic.
- **TypeScript:** Used for the operator console UI and frontend tests.

## Backend Frameworks and Libraries
- **Standard Library:** Heavily used for networking and concurrency.
- **etcd client/v3:** Interface with the authoritative state store.
- **k8s.io/client-go:** Core library for Kubernetes interaction and resource management.
- **gorilla/websocket:** For real-time terminal attach and console updates.

## Frontend Frameworks and Libraries
- **SvelteKit:** High-performance web framework for the operator console.
- **Tailwind CSS & Flowbite:** For rapid UI development with a consistent, professional design.
- **Vite:** Build tool for the frontend.

## Infrastructure and Deployment
- **Kubernetes:** The primary execution and orchestration substrate.
- **Helm:** For packaging and deploying the Smith control plane.
- **Docker:** For building service and replica images.

## Testing and Verification
- **Playwright:** End-to-end testing for the frontend and UI workflows.
- **Godog (Cucumber):** Behavior-driven development (BDD) for system-level acceptance tests.
- **Testify:** Unit and integration testing utility.
