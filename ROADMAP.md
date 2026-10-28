# Platformctl Vision and Roadmap

**Last updated:** 2026-10-28

**Status:** Proposed direction

**Initial audience:** One platform team running a self-hosted installation

## Vision

Platformctl gives engineers an evidence backed explanation of an application's deployment state across environments, and a controlled way to resolve problems.

The primary user is a platform engineer investigating a failed deployment or an environment difference. Platformctl should answer four questions:

- What is deployed?
- What differs from the intended configuration?
- What is failing?
- What should I check or change next?

The App–Environment–Context model connects application definitions, environment configuration, and observed deployment state. The product's value comes from tracing those relationships across Argo CD, Kubernetes, Helm values, and Vault.

A target workflow is:

```text
platformctl context explain checkout-prod

Deployment: degraded
Argo CD: desired revision applied
Kubernetes: 2 of 3 replicas ready
Finding: one pod references a missing Secret key
Evidence: Deployment checkout → Secret checkout-db → key password
Last checked: 18 seconds ago
```

This command and output describe a proposed capability. The first major objective is to make explanations like this accurate, traceable, and useful during an investigation.

## Starting point

The repository has an HTTP gateway, a Context CLI, PostgreSQL manifest storage and status read models, RabbitMQ messaging, integration workers, and a Helm chart. These provide the foundation for the roadmap.

Several gaps affect whether engineers can trust the results:

- Some Context checks return simulated successful results, and Kubernetes workload collection can fall back to simulated data.
- Some integration paths use conventions or placeholder configuration instead of resolving the stored App and Environment manifests.
- The CLI covers Contexts; App and Environment management requires the HTTP API.
- The gateway accepts customer selection headers after Basic Auth without verifying customer membership.
- Correlation IDs and run tracking structures exist, but the complete lifecycle of accepted actions needs durable recording, queryable outcomes, and failure recovery.

Trustworthy observations and dependable execution take priority over adding more integrations.

## Release sequence

The version labels below identify proposed milestones. They do not describe features already released. Dates and owners should be assigned when work is estimated against available team capacity. Each milestone depends on the preceding release gate.

| Order | Milestone | Outcome |
| --- | --- | --- |
| Now | v0.1 — One trustworthy diagnostic workflow | Diagnose one real application through Argo CD and Kubernetes using current evidence. |
| Next | v0.2 — Dependable actions and complete CLI | Manage all manifest types and track authorized actions through a durable lifecycle. |
| Then | v0.3 — Explain environment differences | Trace unexpected differences through Helm values, Vault references, and workloads. |
| Later | v1.0 — Daily operational use | Support routine investigations, notifications, upgrades, and recovery for one platform team. |

## v0.1 — One trustworthy diagnostic workflow

**Outcome:** A new engineer can onboard an application and use platformctl to identify real deployment failures.

Scope:

- Follow one application from its Context through Argo CD to Kubernetes workloads, using explicit resource references.
- Replace simulation in runtime paths with explicit `unknown`, `unavailable`, or `stale` observations. Keep synthetic data in tests and clearly identified demonstrations.
- Attach evidence, source resource identity, revision, and observation timestamps to findings.
- Distinguish workload health from collection failures. Preserve the last successful observation alongside the latest collection error.
- Provide a reproducible local setup, example manifests, and a documented onboarding path.
- Fix configuration and startup issues that prevent that path from working consistently.
- Rotate any deployed RabbitMQ accounts still using the earlier credentials exposed in Git history, and verify recovery after updating their Secrets and restarting services.
- Establish a server-controlled identity boundary for the single team installation; request headers must not grant access to arbitrary customer data.

**Release gate:** A new engineer can onboard one application and correctly diagnose deliberately introduced failures, such as an unavailable image, a missing Secret reference, and an unavailable integration. Losing access to an integration never produces a false healthy result.

**Dependencies:** A representative application, an Argo CD instance, and a Kubernetes test environment where failures can be introduced safely.

## v0.2 — Dependable actions and complete CLI

**Outcome:** Engineers can manage configuration and initiate operations with clear authorization and a durable record of what happened.

Scope:

- Add App and Environment CLI support, manifest validation, and `context explain` backed by the evidence model from v0.1.
- Expose action run queries with queued, running, succeeded, failed, and timed out states, including error details and relevant timestamps.
- Persist accepted work and make publication recoverable. Evaluate a transactional outbox to coordinate database writes with message publication.
- Add publisher confirms, routing failure handling, bounded retries, and a way to inspect work that cannot be processed.
- Make result processing idempotent and protect external actions against unsafe repetition. Recovery must account for uncertainty when an external operation succeeds but its response is lost.
- Record who requested an action, its target, and its outcome in an audit trail.
- Connect team SSO and reader/operator permissions before expanding access. Inspection requires read permissions; changes require explicit operation permissions.

**Release gate:** Worker restarts, broker interruptions, duplicate or out-of-order messages, and external timeouts do not silently lose accepted work or blindly repeat completed destructive actions. Every accepted action reaches a durable, discoverable outcome, including an explicit failure or timeout when completion cannot be established. Authorization tests cover both inspection and changes.

**Dependencies:** The v0.1 resource and identity model, access to the team's identity provider, and integration tests that exercise failures across the database, broker, worker, and external API boundaries.

RabbitMQ's [reliability guidance](https://www.rabbitmq.com/docs/reliability) explains why publisher confirms must be paired with handling for duplicate deliveries during recovery.

## v0.3 — Explain environment differences

**Outcome:** Engineers can determine why an application works in one environment and fails in another.

Scope:

- Resolve actual Helm values and Vault references from stored manifests and their source revisions.
- Compare intended configuration with observed deployment state across environments.
- Allow teams to declare expected differences, such as replica counts or environment-specific endpoints, so those differences do not generate drift findings.
- Trace Secret references through configuration and workloads without displaying or storing secret values in diagnostic results.
- Explain unexpected differences with links to the relevant configuration, resources, and observation times.
- Use real reference validation and environment mappings throughout the correlation workers.

**Release gate:** In representative staging and production scenarios, platformctl explains the relevant configuration or resource difference with evidence. Approved differences are identified correctly, and diagnostic output contains no secret values.

**Dependencies:** Reliable observations and action tracking, representative applications deployed to multiple environments, and access to the actual Helm and Vault configurations.

## v1.0 — Daily operational use

**Outcome:** The platform team uses platformctl during real incidents and deployments and can operate the installation reliably.

Scope:

- Add scheduled refresh with visible freshness and collection errors.
- Keep a useful change history for deployment state, findings, and operations.
- Send targeted notifications for actionable changes, with deduplication and links to supporting evidence.
- Provide a small read-only dashboard for Context health, findings, history, and action outcomes.
- Complete the Helm installation and upgrade path, including migrations, configuration, credentials, and dependency readiness checks.
- Document and exercise PostgreSQL backup and restore, including reconciliation of restored observations and action records with current external state.
- Run integration tests in CI against provisioned dependencies and supported integration versions.

**Release gate:** The team uses platformctl during real investigations and deployments, can upgrade and restore it successfully, and demonstrates a measured reduction in investigation time against its baseline.

**Dependencies:** A sustained internal pilot, operational ownership, and a repeatable upgrade and recovery environment.

## Architectural direction

### Git owns desired configuration

Make Git authoritative for desired App, Environment, and Context configuration. Platformctl imports and validates manifests; PostgreSQL holds indexed configuration, observations, and operation history.

Define imported revision tracking, update conflicts, and the behavior of CLI or API edits before implementing this change. Local edits must have an explicit path back to Git or an explicit temporary override lifecycle. Record this decision during v0.1 so later CLI and integration work follows the same ownership rules.

### Keep the deployment manageable

Evaluate combining the five specialized integration workers into one deployable worker process while retaining their package interfaces and responsibilities. Validate routing, concurrency, and failure handling before consolidation. Split processes when independent scaling, failure isolation, or team ownership justifies the operational cost.

### Every finding has provenance

A finding should identify its source resource, source revision where available, observation time, check outcome, and supporting evidence. Keep desired configuration, observed workload state, and collection health distinguishable throughout storage, aggregation, and presentation.

## Success measures

Establish baselines with a few real applications before committing to numerical performance or availability targets.

| Measure | How to assess it |
| --- | --- |
| Time to first useful diagnosis | Time from starting onboarding to obtaining a correct, evidence supported finding. |
| Time to identify a deployment failure's cause | Compare investigations of representative failures with the team's existing workflow. |
| Findings supported by current evidence | Proportion of findings with traceable sources and observations within a defined freshness window. |
| Accepted actions reaching a recorded terminal state | Proportion reaching succeeded, failed, or timed out, with unresolved runs and their age visible. |
| Operational adoption | Use during real incidents and deployments, with feedback about whether findings changed the next investigative step. |

Agree on freshness windows and pilot scenarios during v0.1. Review the measures at each release gate and use the results to choose subsequent work.

## Deferred investments

Revisit these when there is a demonstrated requirement:

- Service mesh adoption and additional internal networking infrastructure.
- Redis clustering, database partitioning, and other scaling infrastructure.
- Multi-region operation.
- Hosted SaaS, billing, and administration for multiple independent organizations.
- Broad integration expansion beyond the initial Argo CD, Kubernetes, Helm, and Vault workflow.

Keep the roadmap tied to observed investigation and operational needs. Before scheduling an initiative, assign an owner, estimate it against available capacity, identify dependencies, and define the evidence required to pass its release gate.
