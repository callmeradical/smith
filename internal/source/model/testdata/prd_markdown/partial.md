# Partial Import

## Overview

Bring semi-structured markdown into the canonical format.

## Goals

- Capture goals when present.

## Quality Gates

- make ci-local-act

## Stories

- US-001: Parse well-structured stories
  As a maintainer, I want explicit story sections normalized automatically.
  - Acceptance: Parse explicit story IDs.
- US-002: Preserve quality gates
  As an operator, I want gate commands carried into canonical JSON.
  - Depends on: US-001
  - Acceptance: Carry gate commands into JSON.
