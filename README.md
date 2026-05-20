# remetric

Re-metric your stack — find waste in Prometheus, Grafana & Loki.

`remetric` is a read-only doctor for self-hosted Prometheus stacks. Point it at a
Prometheus server and it prints a ranked, actionable list of cardinality
problems with suggested `metric_relabel_configs` fixes.

> Status: **alpha (Phase 2 of v0.1)** — cardinality analyzer + label-pattern
> analyzer + JSON output are wired up. Grafana integration, alert hygiene,
> unused-metric detection, and HTML/Markdown reports come in later phases.

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

## Commands

| Command                            | What it does                                                |
|------------------------------------|-------------------------------------------------------------|
| `remetric doctor`                  | Connectivity + version + permission self-check              |
| `remetric cardinality top`         | List the worst-offending high-cardinality metric/label pairs|
| `remetric cardinality labels`      | Per-metric label inventory (unique counts + sample values)  |
| `remetric cardinality suspicious`  | Flag labels matching unbounded-identifier patterns          |

Global flags (subset; see `--help` for the full list):

- `--prometheus URL` — Prometheus base URL. Env: `REMETRIC_PROMETHEUS_URL`.
- `--prom-token TOK` — Bearer token. Env: `REMETRIC_PROMETHEUS_TOKEN`.
- `--prom-basic-auth USER:PASS` — Basic auth.
- `--prom-max-in-flight N` — Concurrency cap (default 5).
- `--output FORMAT` — `terminal` (default) or `json`.
- `--no-color` — Disable colored output (`NO_COLOR` env also respected).
- `--verbose` — Debug-level slog logging on stderr.

## What does Phase 2 *not* do yet?

- No HTML / Markdown output.
- No Grafana integration; we can't tell you which metrics are unused.
- No `--fail-on` flag for CI integration.
- No goreleaser / Docker image / Homebrew tap.

These ship in later phases (see `.claude/spec.md`).

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
