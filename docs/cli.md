# platformctl CLI

`platformctl` is a command-line client for the Platformctl API Gateway. It manages
contexts, triggers GitOps actions, and queries the aggregated status read model over
HTTP. Source: [`cmd/cli`](../cmd/cli) (entrypoint) and [`internal/cli`](../internal/cli).

## Build & install

```bash
make cli                              # builds ./bin/platformctl
# NOTE: the Makefile defaults to GOOS=linux/GOARCH=amd64 for release images.
# For a binary that runs on your machine, build natively:
go build -o bin/platformctl ./cmd/cli
```

`make build-local` and `make build-all` also produce the CLI alongside the services.

## Configuration

Values resolve in this precedence order (highest first):

1. command-line flags
2. environment variables
3. config file (`$HOME/.platformctl.yaml`, or `--config PATH`)
4. built-in defaults

| Setting | Flag | Env var | Config key | Default |
|---|---|---|---|---|
| Gateway base URL | `--server` | `PLATFORMCTL_SERVER` | `server` | `http://localhost:8080` |
| Output format | `-o`, `--output` | `PLATFORMCTL_OUTPUT` | `output` | `table` |
| Basic-auth user | `--username` | `PLATFORMCTL_USERNAME` | `username` | `admin` |
| Basic-auth password | `--password` | `PLATFORMCTL_PASSWORD` | `password` | `admin` |
| Tenant/customer id | `--customer-id` | `PLATFORMCTL_CUSTOMER_ID` | `customerId` | *(unset)* |
| Bearer token | `--token` | `PLATFORMCTL_TOKEN` | `token` | *(unset)* |
| Skip TLS verify | `--insecure` | `PLATFORMCTL_INSECURE` | `insecure` | `false` |
| Verbose (log HTTP) | `-v`, `--verbose` | — | — | `false` |

Example `~/.platformctl.yaml`:

```yaml
server: https://platformctl.example.com
output: table
username: admin
password: admin
customerId: acme
```

### Authentication

- By default the CLI sends HTTP basic auth (`--username`/`--password`, default `admin`/`admin`) — the only scheme the gateway accepts today.
- `--token` sends `Authorization: Bearer …` instead (and suppresses basic auth). This is for the **planned** JWT auth; the current gateway only understands basic auth, so `--token` will `401` until JWT support lands.
- When `--customer-id` is set, the CLI also sends `X-Customer-ID` and `X-User-ID` headers.

> The gateway currently authenticates every `/api/v1` route with basic auth but does not
> yet populate the tenant/customer context its handlers read, so authenticated requests
> return `401` until that server-side middleware is wired up. The CLI already sends the
> headers above so it works unchanged once the gateway side lands.

## Output formats

Every command supports `-o table` (default, human-readable), `-o json`, and `-o yaml`.
JSON/YAML echo the raw gateway response, so they are ideal for scripting (e.g. piping to `jq`).

## Commands

### `context create [FILE]`

Create a context from a YAML or JSON manifest. The manifest may be a positional argument,
`--file/-f`, or piped on stdin (`-`). It is validated client-side before being sent.

```bash
platformctl context create docs/examples/context.yaml
platformctl context create -f context.yaml
cat context.yaml | platformctl context create -
```

### `context list` (alias `ls`)

List all contexts. Table columns: `NAME`, `APP`, `ENVIRONMENTS`, `CUSTOMER BRANCH`, `CREATED`.

### `context get NAME`

Show a single context, including its deployments.

### `context update NAME [FILE]`

Update a context from a manifest (`-f`, positional, or stdin). The manifest's
`metadata.name` must match `NAME` if present.

### `context delete NAME` (alias `rm`)

Delete a context. Prompts for confirmation unless `--force` is given.

### `context status NAME`

Show the aggregated GitOps read-model status (health, sync and pairing state).
`--watch` re-renders on an interval (`--interval`, default `5s`) until interrupted.

### `context run NAME ACTION`

Trigger a GitOps action. Returns a correlation id to track the published command.

| Action | Aliases | Endpoint |
|---|---|---|
| `sync-apps` | `sync`, `refresh` | synchronize ArgoCD ApplicationSets |
| `validate-environments` | `validate` | validate environment manifests / Vault secrets |
| `correlate-contexts` | `correlate` | correlate the app+environment pairing |
| `correlate-multi-environment` | — | correlate workloads across environments |
| `inspect-manifests` | `inspect` | inspect manifests (`--type app|environment|context|all`) |

```bash
platformctl context run web-app-prod sync-apps
platformctl context run web-app-prod validate
platformctl context run web-app-prod inspect-manifests --type app
```

### `completion [bash|zsh|fish|powershell]`

Generate a shell-completion script.

```bash
source <(platformctl completion bash)
platformctl completion zsh > "${fpath[1]}/_platformctl"
```

### `version`

Print version, commit and build date (injected via `-ldflags` at build time).

## Exit codes

`0` on success; `1` on any error (validation failure, network error, or a non-2xx
response from the gateway). Error messages are written to stderr.

## Endpoint mapping

| Command | Method & path |
|---|---|
| `context create` | `POST /api/v1/contexts` (body `{"context": …}`) |
| `context list` | `GET /api/v1/contexts` |
| `context get` | `GET /api/v1/contexts/{name}` |
| `context update` | `PUT /api/v1/contexts/{name}` |
| `context delete` | `DELETE /api/v1/contexts/{name}` |
| `context run` | `POST /api/v1/contexts/{name}/actions/{action}` |
| `context status` | `GET /api/v1/gitops/contexts/{name}/status` |

See [context-model.md](data-models/context-model.md) for the manifest structure and
[`docs/examples/`](examples/) for ready-to-use sample manifests.
