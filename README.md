# Platformctl

GitOps monitoring platform for applications deployed via ArgoCD ApplicationSets, Helm umbrella charts, and Vault-backed secrets. It watches the whole delivery chain — ApplicationSet → generated Applications → Helm releases → VaultStaticSecrets → pod environment variables — and materializes a per-context, per-environment health read model behind a REST API.

**Status (2026-07-02):** stage deploys are green — all 7 services build in CI, deploy via Helm, and pass their Kubernetes probes on the DigitalOcean cluster. Prod has not been deployed yet. A `platformctl` CLI now wraps the REST API (`make cli`; see [CLI](#cli) below). Sections below marked *(planned)* are aspirational.

---

## Architecture

Event-driven Go microservices. The gateway accepts HTTP requests and publishes command messages to RabbitMQ; worker services consume commands, inspect external systems (Kubernetes, Git, ArgoCD, Vault), and publish results; the aggregator folds results into a Postgres read model that the gateway serves back out.

```
HTTP client ──▶ gateway ──▶ RabbitMQ ──▶ worker services ──▶ RabbitMQ ──▶ gitops-aggregator
                  │                       (app-sync, kube,                      │
                  │                        git-branch, validation,              ▼
                  └──────────── read ◀─── correlation)                  PostgreSQL read model
```

### Services (`cmd/`)

| Service | Role |
|---|---|
| `gateway` | HTTP API (Gin). CRUD for contexts/apps/environments, status queries, and action endpoints that publish commands. Runs schema migrations on startup — the only service that uses the **direct** DB connection. |
| `gitops-aggregator` | Consumes result events and materializes the multi-environment read model in Postgres. |
| `app-sync-svc` | ApplicationSet and generated-application sync monitoring. |
| `multi-environment-kube-svc` | Collects Kubernetes state across environments/namespaces (consumer-only). |
| `customer-git-branch-svc` | Tracks customer branches and correlates per-env Helm values files (consumer-only). |
| `environment-validation-svc` | Cross-environment validation and compliance checks. |
| `context-correlation-svc` | Pairs contexts and maintains relationships between them (consumer-only). |
| `test-service` | Dev harness, not deployed. |

Every service serves liveness/readiness on the health port (`/health`, `/ready`, default `:8081`) and Prometheus metrics on the metrics port (default `:9090`). The three consumer-only services run the same health server even though they take no other HTTP traffic — the kubelet kills them otherwise.

### Infrastructure

- **PostgreSQL** — external (DigitalOcean managed). Contexts, audit log, read model. Migrations in [migrations/](migrations/).
- **RabbitMQ** — in-cluster, deployed by the Helm chart (`rabbitmq.enabled`). Command and result queues.
- **Redis** *(planned)* — caching layer, not used yet.

### API surface

Mounted under `/api/v1`:

- CRUD: `contexts`, `apps`, `environments`
- Actions (publish commands): `POST /contexts/:name/actions/{sync-apps, validate-environments, correlate-contexts, correlate-multi-environment, inspect-manifests}`
- Status (read model, under the `/api/v1/gitops` prefix): `GET /gitops/contexts/:name/status`, per-app and per-environment status, `GET /gitops/contexts/:name/environments/:env/vault/status`, `GET /gitops/health/overview`

Routes are registered in `cmd/gateway/main.go`.

---

## Repo layout

```
cmd/                  # one main.go per service (see table above)
internal/             # private packages: config, events (RabbitMQ), storage,
                      #   handlers, readmodel, observability, validation, ...
pkg/api/              # shared API message and type definitions
migrations/           # SQL migrations (run by gateway on startup, or make db-migrate)
charts/platformctl/   # Helm chart + values-stage.yaml / values-prod.yaml
.github/workflows/    # build-and-push, deploy, test, cleanup
docs/                 # phases/, adr/, data-models/
scripts/              # build-images.sh, cleanup.sh
test/                 # integration tests
```

---

## Installation

```bash
git clone https://github.com/kriipke/platformctl.git
cd platformctl
make build                 # compile all service binaries into ./bin
go install ./cmd/cli       # put the `platformctl` CLI on your PATH
```

Prebuilt container images for every service are published to
`ghcr.io/kriipke/platformctl-<service>`; see [Deployment](#deployment) to run
the full stack on Kubernetes.

---

## Development

Prereqs: Go 1.24+, Docker, `make`. A reachable PostgreSQL and RabbitMQ for anything beyond compiling.

```bash
make build              # build all service binaries
make test               # go test -race -cover ./...
make lint               # golangci-lint
make db-migrate         # requires DATABASE_URL and the golang-migrate CLI
                        #   (004 has no down migration; db-migrate-down stops at 003)
make docker-build       # build container images
make help               # everything else
```

Local dependencies come from [deployments/docker-compose.dev.yml](deployments/docker-compose.dev.yml):

```bash
make dev-up             # start Postgres 14 + RabbitMQ (management UI on :15672)
make dev-logs           # tail their logs
make dev-down           # stop them

export DATABASE_URL='postgres://platformctl:platformctl@localhost:5432/platformctl?sslmode=disable'
# RABBITMQ_URL can be left unset — the config default amqp://localhost:5672/ works as-is
```

If another project already holds a port, override it, e.g. `POSTGRES_HOST_PORT=55432 make dev-up` (and adjust `DATABASE_URL` to match).

Key environment variables (parsed in `internal/config`): `DATABASE_URL`, `RABBITMQ_URL`, `PORT`, `HEALTH_CHECK_PORT`, `LOG_LEVEL`, `ENABLE_METRICS`. See [docs/adr/](docs/adr/) for configuration decisions.

---

## CLI

`platformctl` is a thin, scriptable client for the gateway's REST API (`cmd/cli`, `internal/cli`).

```bash
make cli                # build ./bin/platformctl (honours GOOS/GOARCH; omit them for a native build)
go build -o bin/platformctl ./cmd/cli   # native build for local use
```

```bash
platformctl context create docs/examples/context.yaml   # create from a YAML/JSON manifest (also -f / stdin '-')
platformctl context list                                # NAME / APP / ENVIRONMENTS / BRANCH / CREATED
platformctl context get web-app-prod
platformctl context status web-app-prod                 # aggregated GitOps read-model status (--watch to poll)
platformctl context run web-app-prod sync-apps          # trigger an action (aliases: sync, validate, inspect, ...)
platformctl context run web-app-prod inspect-manifests --type app
platformctl context delete web-app-prod                 # --force to skip the prompt
platformctl completion zsh                              # bash | zsh | fish | powershell
```

Every command accepts `-o table|json|yaml`. Configuration resolves flags → env (`PLATFORMCTL_SERVER`,
`PLATFORMCTL_OUTPUT`, `PLATFORMCTL_USERNAME`, `PLATFORMCTL_PASSWORD`, `PLATFORMCTL_CUSTOMER_ID`,
`PLATFORMCTL_TOKEN`, `PLATFORMCTL_INSECURE`) → config file (`$HOME/.platformctl.yaml` or `--config`) →
defaults (`--server http://localhost:8080`, basic auth `admin`/`admin`). Manifests are validated
client-side before they are sent. See [docs/cli.md](docs/cli.md) for the full reference.

### Config file

Rather than passing `--server`/`--username`/`--password` on every call, drop a config file at
`$HOME/.platformctl.yaml` (loaded automatically) and point it at the gateway with the admin basic-auth
credentials from the `platformctl-gateway-admin` secret (see
[One-time setup per environment](#one-time-setup-per-environment)):

```yaml
server: https://api-stage.opsdash.dev   # gateway base URL — the CLI appends /api/v1
username: admin
password: "REPLACE-WITH-platformctl-gateway-admin-PASSWORD"
output: table                            # table | json | yaml
# customerId: acme                       # optional tenant; sent as X-Customer-ID
# insecure: false                        # skip TLS verification — dev / self-signed only
```

```bash
chmod 600 ~/.platformctl.yaml            # holds a plaintext password — keep it private, never commit it
platformctl context list                 # now works with no flags
```

JSON is also accepted — the same keys parse from e.g. `~/.platformctl.json`, but that filename is **not**
auto-loaded, so pass it explicitly: `platformctl --config ~/.platformctl.json context list`. Do **not**
set `token`: a bearer token takes precedence over basic auth, but the gateway only accepts basic auth
today, so a token yields `401`.

---

## Deployment

One Helm chart ([charts/platformctl](charts/platformctl/)), one values file per environment, everything on a single DigitalOcean Kubernetes cluster isolated by namespace:

| Environment | Namespace | Values file |
|---|---|---|
| stage | `platformctl-stage` | `charts/platformctl/values-stage.yaml` |
| prod | `platformctl-prod` | `charts/platformctl/values-prod.yaml` |

Postgres is external (DO managed); RabbitMQ is bundled in-cluster. The chart **references** its database and gateway-admin secrets per namespace but never creates them.

### One-time setup per environment

**Database roles** — on the managed Postgres as admin:

```sql
CREATE ROLE platformctl_stage LOGIN PASSWORD '<PASSWORD>';
CREATE DATABASE platformctl_stage OWNER platformctl_stage;
-- repeat for platformctl_prod
```

DigitalOcean exposes port `25060` (direct) and `25061` (PgBouncer pool). The gateway runs migrations on startup and must use the **direct** connection; every other service uses the **pool**. Create the pool in the DO console or with `doctl databases pool create` — the `_pool` suffix is a PgBouncer pool name, not a separate database.

**Secrets** — two per namespace, each holding a base64 `DATABASE_URL`:

| Secret | Connection | Read by |
|---|---|---|
| `platformctl-credentials` | direct `:25060` | gateway only |
| `platformctl-pool-credentials` | pool `:25061` | all other services |

```bash
kubectl -n platformctl-stage create secret generic platformctl-credentials \
  --from-literal=DATABASE_URL='postgresql://platformctl_stage:<PASSWORD>@<SUBDOMAIN>.db.ondigitalocean.com:25060/platformctl_stage?sslmode=require'

kubectl -n platformctl-stage create secret generic platformctl-pool-credentials \
  --from-literal=DATABASE_URL='postgresql://platformctl_stage:<PASSWORD>@<SUBDOMAIN>.db.ondigitalocean.com:25061/platformctl_stage_pool?sslmode=require'
```

**Gateway admin credentials** — the gateway guards every `/api/v1` route with HTTP basic auth read from the `platformctl-gateway-admin` secret (keys `username` / `password`). It fails **closed**: without the secret the gateway returns `503` on `/api/v1` instead of falling back to a default credential, so create it before the first deploy:

```bash
kubectl -n platformctl-stage create secret generic platformctl-gateway-admin \
  --from-literal=username=admin \
  --from-literal=password="$(openssl rand -base64 24)"
```

Rotate the password by replacing the secret and restarting the gateway — the value is read only at pod start:

```bash
kubectl -n platformctl-stage create secret generic platformctl-gateway-admin \
  --from-literal=username=admin --from-literal=password="$(openssl rand -base64 24)" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n platformctl-stage rollout restart deploy/platformctl-gateway
```

Read the current password back with `kubectl -n platformctl-stage get secret platformctl-gateway-admin -o jsonpath='{.data.password}' | base64 -d`, and use it as the CLI `password` (see [CLI](#cli)).

For manual deploys the `ghcr-pull` image-pull secret must also exist in the namespace — CI creates it from `GHCR_PULL_PAT`, so it's only missing on a namespace CI has never touched.

### CI/CD (GitHub Actions)

`deploy.yml` runs `helm upgrade --install --atomic`; deploys are serialized per environment:

| Trigger | Deploys to |
|---|---|
| push to `main` | stage (image tag `latest`) |
| push tag `v*.*.*` | prod (image tag = the git tag) |
| Run workflow (manual) | choose `stage`/`prod` + optional tag |

Images come from the `Build and Push Container Images` workflow on the same event; the deploy job waits for the gateway image before proceeding, then waits for all rollouts after the Helm release lands.

Required repository secrets:

| Secret | Purpose |
|---|---|
| `DIGITALOCEAN_ACCESS_TOKEN` | used by `doctl` to fetch the cluster kubeconfig in CI |
| `GHCR_PULL_PAT` | PAT with `read:packages`; creates the in-namespace `ghcr-pull` pull secret |

### Manual deploy / local render

```bash
doctl kubernetes cluster kubeconfig save <cluster>

helm upgrade --install platformctl charts/platformctl \
  --namespace platformctl-stage \
  -f charts/platformctl/values-stage.yaml \
  --atomic --timeout 10m

# render without touching the cluster
helm template platformctl charts/platformctl \
  -n platformctl-stage -f charts/platformctl/values-stage.yaml
```

RBAC (ServiceAccounts + read-only ClusterRoles) ships with the chart and is created on install; ClusterRole names are namespace-suffixed so stage and prod coexist on one cluster. Toggle with `rbac.create`.

---

## Verify & troubleshoot

```bash
NS=platformctl-stage

kubectl get pods -n $NS                          # expect 7/7 Ready (+ RabbitMQ)
kubectl port-forward svc/platformctl-gateway 8080:80 -n $NS
curl http://localhost:8080/health

kubectl logs -l app=gateway -n $NS
kubectl get events -n $NS --sort-by='.lastTimestamp'

# RabbitMQ management UI
kubectl port-forward svc/rabbitmq 15672:15672 -n $NS

# follow a request end-to-end
kubectl logs -f deploy/platformctl-app-sync-svc -n $NS | grep correlation_id
```

Common failure modes: a consumer pod crash-looping usually means its health server isn't binding (probe failure), a bad `DATABASE_URL` secret, or RabbitMQ not up yet; the gateway failing at startup is usually the direct-vs-pool connection mixup (migrations can't run through PgBouncer).

---

## Roadmap *(planned)*

- Wire the gateway's customer/tenant auth middleware so the CLI (and any API client) can get past `401`
- First prod deploy (tag-triggered pipeline is in place and untested against prod)
- Deeper Vault secret-sync validation — a Vault status endpoint exists today, but the dedicated Vault integration service from the phase plan (real-time VaultStaticSecret ↔ pod env correlation) isn't built
- New Relic integration service
- Redis caching, JWT auth, RBAC/tenant isolation

See [ROADMAP.md](ROADMAP.md) for the full plan and [docs/phases/](docs/phases/) for the phase-by-phase design docs.

---

## Further reading

- [docs/cli.md](docs/cli.md) — `platformctl` CLI reference
- [CLAUDE.md](CLAUDE.md) — development guide (phases, schema, message envelope, conventions). Its project-layout section predates the current structure — trust this README for what's actually in `cmd/` and `internal/`.
- [docs/adr/](docs/adr/) — architectural decision records
- [docs/data-models/](docs/data-models/) — context YAML, API schemas, database schema
- [TEST_SUITE.md](TEST_SUITE.md) — test suite documentation

Licensed under the [MIT License](LICENSE).
