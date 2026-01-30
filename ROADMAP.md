# Eidos Roadmap

This roadmap tracks remaining work for Eidos v2 launch and future enhancements.

## Structure

| Section | Description |
|---------|-------------|
| **Remaining MVP Work** | Tasks blocking v2 launch |
| **Backlog** | Post-launch enhancements by priority |
| **Completed** | Delivered capabilities (reference only) |

---

## Remaining MVP Work

### MVP Recipe Matrix Completion

**Status:** In progress (5 of ~20 recipes complete)

Expand recipe coverage for MVP platforms and accelerators.

**Current:**
- EKS + GB200 + Training
- EKS + GB200 + Ubuntu + Training
- H100 + Ubuntu + Inference

**Needed:**

| Platform | Accelerator | Intent | Status |
|----------|-------------|--------|--------|
| GKE | H100 | Training | Not started |
| GKE | H100 | Inference | Not started |
| GKE | GB200 | Training | Not started |
| AKS | H100 | Training | Not started |
| AKS | H100 | Inference | Not started |
| OKE | H100 | Training | Not started |
| OKE | H100 | Inference | Not started |
| OKE | GB200 | Training | Not started |
| EKS | H100 | Training | Not started |
| EKS | A100 | Training | Not started |

**Acceptance:** each validates and generates bundles.

---

### Validator Enhancements

**Status:** Core complete, advanced features pending

**Implemented:**
- Constraint evaluation against snapshots
- Component health checks
- Validation result reporting

**Needed:**

| Feature | Description | Priority |
|---------|-------------|----------|
| NCCL fabric validation | Deploy test job, verify GPU-to-GPU communication | P0 |
| CNCF AI conformance | Generate conformance report | P1 |
| Remediation guidance | Actionable fixes for common failures | P1 |

**Acceptance:** `eidos validate --fabric` and `eidos validate --conformance ai` produce valid output.

---

### E2E Deployment Validation

**Status:** Partial

Validate bundler output deploys successfully on target platforms.

| Platform | Script Deploy | ArgoCD Deploy |
|----------|---------------|---------------|
| EKS | Not validated | Not validated |
| GKE | Not validated | Not validated |
| AKS | Not validated | Not validated |

**Acceptance:** At least one successful deployment per platform with both deployers.

---

## Backlog

Post-launch enhancements organized by priority.

### P1 — High Value

#### Expand Recipe Coverage

Extend beyond MVP platforms and accelerators.

- Self-managed Kubernetes support
- Additional cloud providers (Oracle OCI, Alibaba Cloud)
- Additional accelerators (L40S, future architectures)
- Prioritized recipe backlog with components

#### New Bundlers

Migrate capabilities from Eidos v1 playbooks.

| Bundler | Description |
|---------|-------------|
| NIM Operator | NVIDIA Inference Microservices deployment |
| KServe | Inference serving configurations |
| Nsight Operator | Cluster-wide profiling and observability |
| Monitoring | Metrics, logging, alerting components |
| Storage | GPU workload storage configurations |

#### Recipe Creation Tooling

Simplify recipe development and contribution.

- Snapshot-to-recipe transformation
- Interactive recipe builder CLI
- Recipe contribution workflow (PR template, validation gates)

---

### P2 — Medium Value

#### Configuration Drift Detection

Detect when clusters diverge from recipe-defined state.

- `eidos diff` command for snapshot comparison
- Scheduled drift detection via CronJob
- Alerting integration for drift events

#### Enhanced Skyhook Integration

Deeper OS-level node optimization.

- OS-specific overlays (Ubuntu, RHEL, Amazon Linux)
- Consistent Skyhook recipe per OS
- Automated node configuration validation

---

### P3 — Future

#### Additional API Interfaces

Programmatic integration options.

- gRPC API for high-performance access
- GraphQL API for flexible querying
- Multi-tenancy support

## Revision History

| Date | Change |
|------|--------|
| 2026-01-26 | Reorganized: removed completed items, streamlined structure |
| 2026-01-17 | Restructured to JIRA format with Initiatives and Epics |
| 2026-01-06 | Updated structure |
| 2026-01-05 | Added Opens section based on architectural decisions |
| 2026-01-01 | Initial comprehensive roadmap |
