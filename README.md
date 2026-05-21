# remetric

Re-metric your stack — find waste in Prometheus, Grafana & Loki.

`remetric` is a read-only doctor for self-hosted Prometheus stacks. Point it at a
Prometheus server and it prints a ranked, actionable list of cardinality
problems with suggested `metric_relabel_configs` fixes.

> Status: **alpha (Phase 3 of v0.1)** — cardinality, label-pattern, and
> unused-metric analyzers are wired up. JSON output + Grafana integration +
> unified `remetric scan` command shipped. Alert hygiene and HTML/Markdown
> reports come in later phases.

## Install

### One-line install

```bash
curl -sSL https://raw.githubusercontent.com/remetric-dev/remetric/main/install.sh | sh
```

Installs the latest release into `$HOME/.local/bin`. Override with
`REMETRIC_INSTALL_DIR=/usr/local/bin sh install.sh` (may need `sudo`) or pin a
specific version via `REMETRIC_VERSION=v0.1.0 sh install.sh`.

### Docker

Multi-arch image (`linux/amd64`, `linux/arm64`).

Pick the invocation that matches where Prometheus is running:

```bash
# Prometheus on the host (Linux) — share the host network
docker run --rm --net=host \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus http://127.0.0.1:9090

# Prometheus on the host (macOS / Windows Docker Desktop) — use the magic DNS name
docker run --rm \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus http://host.docker.internal:9090

# Prometheus reachable on the public internet or a corporate URL
docker run --rm \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus https://prom.example.com

# Prometheus in the same docker network (compose / k8s) — use the service name
docker run --rm --network my-prom-net \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus http://prometheus:9090
```

### Manual download

GitHub Releases ship signed tarballs and a `checksums.txt`. See
[https://github.com/remetric-dev/remetric/releases](https://github.com/remetric-dev/remetric/releases).

## 60-second quickstart

```bash
git clone https://github.com/remetric-dev/remetric
cd remetric
make build

# in another shell — spin up an e2e Prometheus stack
make e2e-up
sleep 10

./bin/remetric doctor      --prometheus http://localhost:9090
./bin/remetric cardinality top --prometheus http://localhost:9090

make e2e-down
```

## Label-pattern analysis (Phase 2)

Find labels whose names look like unbounded identifiers (`user_id`, `trace_id`,
`path`, …) and rank them by uniqueness:

```bash
remetric cardinality suspicious \
  --prometheus http://localhost:9090 \
  --min-severity medium
```

Inspect the labels of a single metric, sorted by unique value count:

```bash
remetric cardinality labels \
  --metric http_requests_total \
  --prometheus http://localhost:9090
```

Both commands accept `--output json` for machine-readable output (schema in
`.claude/spec.md` §5.5).

## Unused-metric detection (Phase 3)

Diff ingested metrics against everything Grafana, alert rules, and recording
rules actually reference. Anything left over is a candidate to drop.

```bash
remetric metrics unused \
  --prometheus http://localhost:9090 \
  --grafana http://localhost:3000
```

`--grafana-token TOKEN` uses a bearer (service-account API key);
`--grafana-basic-auth user:pass` for basic auth.

Run every analyzer in one shot:

```bash
remetric scan \
  --prometheus http://localhost:9090 \
  --grafana http://localhost:3000
```

`scan` emits a `findings.Report` (see spec §5.5) — combine with `--output json`
for CI.

## VictoriaMetrics support

`remetric` works against VictoriaMetrics out of the box. The Prometheus
HTTP API surface VM exposes is auto-detected on first call. Backend
detection is logged once; override via `--backend=victoria` (or
`prometheus`, `auto`) if needed.

````bash
# Single-binary VM (default port 8428)
remetric scan --prometheus http://vm:8428

# VM cluster fronted by vmauth, vmalert separate
remetric scan \
  --prometheus https://vmauth.example.com:8427 \
  --prom-token "$VMAUTH_TOKEN" \
  --vmalert http://vmalert:8880

# Force VM mode (skip auto-detection)
remetric scan --prometheus http://vm:8428 --backend victoria
````

### vmalert

`/api/v1/rules` is served by `vmalert`, not `vmselect`. Without
`--vmalert`, `metrics unused` and `scan` warn with `rules unavailable`
and may report false-positives for metrics referenced only by recording
rules. Point `--vmalert` at the vmalert HTTP listener (default `:8880`)
to get full coverage. Auth flags `--vmalert-token` / `--vmalert-basic-auth`
exist for split-credential setups; if omitted, vmalert inherits auth from
`--prom-token` / `--prom-basic-auth`.

### Known limitations

- `doctor` shows `retention: n/a` — VM does not expose
  `/api/v1/status/runtimeinfo`.
- `cardinality top` derives `numSeries` by summing
  `seriesCountByMetricName` (VM does not return `headStats`).
- Cortex/Mimir-style multi-tenancy headers (`X-Scope-OrgID`) are not
  supported; URL-prefix-based tenant routing through `vmauth` works.

## Commands

| Command                            | What it does                                                |
|------------------------------------|-------------------------------------------------------------|
| `remetric doctor`                  | Connectivity + version + permission self-check              |
| `remetric cardinality top`         | List the worst-offending high-cardinality metric/label pairs|
| `remetric cardinality labels`      | Per-metric label inventory (unique counts + sample values)  |
| `remetric cardinality suspicious`  | Flag labels matching unbounded-identifier patterns          |
| `remetric metrics unused`          | Ingested ∖ used metrics (needs Grafana for dashboard coverage)|
| `remetric scan`                    | Run every available analyzer, emit a unified Report         |

Global flags (subset; see `--help` for the full list):

- `--prometheus URL` — Prometheus base URL. Env: `REMETRIC_PROMETHEUS_URL`.
- `--prom-token TOK` — Bearer token. Env: `REMETRIC_PROMETHEUS_TOKEN`.
- `--grafana URL` — Grafana base URL. Env: `REMETRIC_GRAFANA_URL`.
- `--grafana-token TOK` — Grafana service-account API key. Env: `REMETRIC_GRAFANA_TOKEN`.
- `--grafana-basic-auth USER:PASS` — Basic auth for Grafana.
- `--grafana-tls-skip-verify` — Skip TLS verification for Grafana.
- `--backend {auto|prometheus|victoria}` — backend dialect. Env: `REMETRIC_BACKEND`.
- `--vmalert URL` — vmalert base URL for /api/v1/rules. Env: `REMETRIC_VMALERT_URL`.
- `--vmalert-token TOK` — Bearer for vmalert (inherits from --prom-token if unset). Env: `REMETRIC_VMALERT_TOKEN`.
- `--vmalert-basic-auth USER:PASS` — Basic auth for vmalert (inherits from --prom-basic-auth if unset).
- `--vmalert-tls-skip-verify` — Skip TLS verify for vmalert.
- `--prom-basic-auth USER:PASS` — Basic auth.
- `--prom-max-in-flight N` — Concurrency cap (default 5).
- `--output FORMAT` — `terminal` (default) or `json`.
- `--no-color` — Disable colored output (`NO_COLOR` env also respected).
- `--verbose` — Debug-level slog logging on stderr.

## What's still missing in v0.1

- No alert hygiene analyzer (never-fired / always-firing alerts) — Phase 4.
- No HTML / Markdown output — Phase 4.
- No dashboard sprawl analyzer — Phase 5.
- No `--fail-on` flag for CI integration.
- No Homebrew tap (binaries + Docker image already ship; see Install above).

These land in subsequent phases (see `.claude/spec.md`).

## Building from source

```bash
make build       # static binary at ./bin/remetric
make test        # unit tests
make e2e-up      # docker compose Prometheus + node-exporter
make e2e         # e2e smoke tests
make e2e-down
make fmt vet lint vuln  # tooling
```

## License

Apache 2.0. See `LICENSE`.
