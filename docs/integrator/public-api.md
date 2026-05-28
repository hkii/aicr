# Public API Surface

AICR is both a CLI and a Go library. This page documents the
stability contract for every exported Go package. External consumers
should prefer the top-level `github.com/NVIDIA/aicr` facade described
in the [Go library integration guide](./go-library.md).

## Stability tiers

| Tier | Meaning |
|------|---------|
| **Public (stable)** | Covered by semver; breaking changes only in major bumps. |
| **Public (evolving)** | Exported today but may change in minor bumps. Pin and audit on upgrade. |
| **Internal** | Treated as implementation detail. May change without notice. |

## Package matrix

| Package | Tier | Purpose |
|---------|------|---------|
| `github.com/NVIDIA/aicr` | **Public (stable)** | Top-level facade: `Client`, `NewClient`, request/result types, source constructors. |
| `pkg/recipe` | Public (evolving) | Recipe resolution, criteria, overlay system, component registry. |
| `pkg/bundler` | Public (evolving) | Per-component Helm/Kustomize bundle generation. |
| `pkg/validator` | Public (evolving) | Constraint evaluation, four-phase validation. |
| `pkg/collector` | Public (evolving) | Observed state collection from clusters. |
| `pkg/measurement` | Public (evolving) | Typed measurement model used by collectors and validators. |
| `pkg/version` | Public (evolving) | Semver constraint evaluation. |
| `pkg/errors` | Public (evolving) | Structured errors with error codes. Consumed at API boundaries. |
| `pkg/defaults` | Public (evolving) | Shared timeout and limit constants. |
| `pkg/component` | Internal | Bundler utilities and test helpers. |
| `pkg/constraints` | Internal | Constraint type definitions. |
| `pkg/snapshotter` | Internal | Snapshot orchestration. |
| `pkg/serializer` | Internal | YAML/JSON serialization helpers. |
| `pkg/manifest` | Internal | Helm-compatible manifest rendering. |
| `pkg/evidence` | Internal | Conformance evidence capture. |
| `pkg/trust` | Internal | Sigstore / provenance integration. |
| `pkg/k8s` | Internal | Kubernetes client utilities. |
| `pkg/oci` | Internal | OCI registry helpers. |
| `pkg/logging` | Internal | Logging setup. |
| `pkg/header` | Internal | HTTP header helpers. |
| `pkg/build` | Internal | Build-time metadata. |
| `pkg/server` | Internal | HTTP API server implementation. |
| `pkg/api` | Internal | Server-side REST handlers (server-internal; consumers use the HTTP API, not the Go types). |
| `pkg/cli` | Internal | CLI command implementations. |

## Recommended consumption pattern

1. Use `github.com/NVIDIA/aicr` for all library integration by default.
2. If the facade does not yet expose a feature you need, open an issue
   against [NVIDIA/aicr](https://github.com/NVIDIA/aicr) describing the
   missing capability — we'd rather extend the facade than have
   external consumers hard-couple to evolving subpackages.
3. If you must import a `Public (evolving)` subpackage, pin AICR to a
   patch version and audit diffs when upgrading.
4. Never import a package marked `Internal` — upgrades will break you.

## See also

- [Go library integration guide](./go-library.md)
- [`provider-nvidia`](https://github.com/NVIDIA/provider-nvidia) —
  reference external consumer
