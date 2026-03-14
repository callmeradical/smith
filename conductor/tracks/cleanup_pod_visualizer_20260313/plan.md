# Plan: Remove Pod-Visualizer and Pod-Visualizer-Testing

## Objective
Remove the `pod-visualizer` and `pod-visualizer-testing` directories and all related documentation and references from the project.

## Key Files & Context
- `pod-visualizer/`: Directory containing the pod-visualizer project.
- `pod-visualizer-testing/`: Directory containing testing resources for pod-visualizer.
- `docs/ui-style-pod-visualizer.md`: Style contract documentation.
- `docs/index.md`: Links to the style contract.
- `docs/requirements-fr-nfr.md`: Reference to the style contract.

## Implementation Steps

1.  **Remove Directories**:
    -   Delete `pod-visualizer/`.
    -   Delete `pod-visualizer-testing/`.

2.  **Clean up Documentation**:
    -   Delete `docs/ui-style-pod-visualizer.md`.
    -   Remove the link `[Operator Console UI Style Contract](ui-style-pod-visualizer.md)` from `docs/index.md`.
    -   Remove the reference to `docs/ui-style-pod-visualizer.md` from `docs/requirements-fr-nfr.md`.

3.  **Verification**:
    -   Verify that no references to `pod-visualizer` remain in the codebase using `grep_search`.
    -   Verify that the documentation still builds (if applicable).
    -   Verify that `make ci-local` still passes.

## Verification & Testing
- Run `grep -r "pod-visualizer" .` and ensure no matches outside of `.git`.
- Run `make ci-local` to ensure no build breakage.
