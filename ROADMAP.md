# Platformctl roadmap

**Updated:** 2026-10-23
**Status:** Proposed backlog; scope and priorities need owner confirmation.

This list captures plausible work based on the current repository. Estimates are rough engineering time, not a commitment or a claim that the work is already scheduled. The [README](README.md) records the last verified deployment status; [docs/phases/](docs/phases/) preserves earlier design notes.

## Near-term candidates

| Work | Scope and completion check | Estimate |
|---|---|---:|
| Finish gateway tenant authentication | Connect authenticated requests to the tenant context expected by handlers; cover allowed and denied CLI/API requests with tests. | 6–10 hours |
| Verify a production deployment | Provision required namespace secrets, deploy a tagged image, check health and rollback behavior, and document the result. | 6–10 hours |
| Expand Vault status checks | Compare VaultStaticSecret sync state with the application and pod environment state; add failure cases and tests. | 8–12 hours |
| Improve broker credential handling | Migrate existing RabbitMQ accounts away from the credential exposed in Git history, document rotation, and verify recovery after restart. | 4–6 hours |
| Make the read model more reliable | Test result replay and out-of-order events; address any stale or duplicate status updates found. | 8–12 hours |

These candidates total roughly 32–50 hours. Confirm which are work you actually intend to do before citing them in a project submission.

## Later ideas

- New Relic integration for application health signals.
- Redis caching with measured cache hit rate and invalidation behavior.
- Deeper RBAC and tenant isolation checks.
- Disaster recovery exercises for Postgres and RabbitMQ.
