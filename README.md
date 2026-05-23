# remetric

[![CI](https://github.com/remetric-dev/remetric/actions/workflows/ci.yml/badge.svg)](https://github.com/remetric-dev/remetric/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/remetric-dev/remetric)](https://github.com/remetric-dev/remetric/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/remetric-dev/remetric)](https://goreportcard.com/report/github.com/remetric-dev/remetric)
[![Go Version](https://img.shields.io/github/go-mod/go-version/remetric-dev/remetric)](go.mod)
[![License](https://img.shields.io/github/license/remetric-dev/remetric)](LICENSE)

Re-metric your stack - find waste in Prometheus, Grafana & Loki.

`remetric` is a read-only doctor for self-hosted Prometheus stacks. Point it at a
Prometheus server and it prints a ranked, actionable list of cardinality
problems with suggested `metric_relabel_configs` fixes.

> Status: **alpha** - cardinality, label-pattern, unused-metric,
> alert-hygiene, and dashboard-hygiene (broken-panel) analyzers are
> wired up. JSON output, Grafana integration, unified `remetric scan`,
> and HTML/Markdown reports shipped.

![remetric demo](demo/remetric.gif)

The demo above runs against the [`demo/`](demo/) docker-compose stack
(Prometheus + Grafana + a deliberately misbehaving `cardinality-bomb`
service). Reproduce with `cd demo && docker compose up -d`.

## Install

### One-line install

```bash
curl -sSL https://remetric.dev/install.sh | sh
```

Installs the latest release into `$HOME/.local/bin`. Override with
`REMETRIC_INSTALL_DIR=/usr/local/bin sh install.sh` (may need `sudo`) or pin a
specific version via `REMETRIC_VERSION=v0.1.0 sh install.sh`.

### Homebrew (macOS / Linux)

```bash
brew install remetric-dev/tap/remetric
```

Or, two-line for shorter subsequent invocations:

```bash
brew tap remetric-dev/tap
brew install remetric
```

The formula is auto-published on each release from
[`remetric-dev/homebrew-tap`](https://github.com/remetric-dev/homebrew-tap).

### Docker

Multi-arch image (`linux/amd64`, `linux/arm64`).

Pick the invocation that matches where Prometheus is running:

```bash
# Prometheus on the host (Linux) - share the host network
docker run --rm --net=host \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus http://127.0.0.1:9090

# Prometheus on the host (macOS / Windows Docker Desktop) - use the magic DNS name
docker run --rm \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus http://host.docker.internal:9090

# Prometheus reachable on the public internet or a corporate URL
docker run --rm \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus https://prom.example.com

# Prometheus in the same docker network (compose / k8s) - use the service name
docker run --rm --network my-prom-net \
  ghcr.io/remetric-dev/remetric:latest \
  doctor --prometheus http://prometheus:9090
```

### Manual download

GitHub Releases ship signed tarballs and a `checksums.txt`. See
[https://github.com/remetric-dev/remetric/releases](https://github.com/remetric-dev/remetric/releases).

## 60-second quickstart

Two paths, depending on what you have:

**Option A - all-in-one demo (Docker only).** Spins up a synthetic stack
(Prometheus + Grafana + a misbehaving metric) and writes an HTML report:

```bash
git clone https://github.com/remetric-dev/remetric
cd remetric/demo
docker compose up
# wait ~45s, then:
open output/report.html      # macOS
xdg-open output/report.html  # Linux
```

See [`demo/README.md`](demo/README.md) for what's in the stack and how to poke at it.

**Option B - local binary against an e2e stack.**

```bash
git clone https://github.com/remetric-dev/remetric
cd remetric
make build

# in another shell - spin up an e2e Prometheus stack
make e2e-up
sleep 10

./bin/remetric doctor      --prometheus http://localhost:9090
./bin/remetric cardinality top --prometheus http://localhost:9090

make e2e-down
```

## Label-pattern analysis

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

Both commands accept `--output json` for machine-readable output.

## Unused-metric detection

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

`scan` emits a `findings.Report` envelope (target, overview, findings,
warnings) - combine with `--output json` for CI.

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

- `doctor` shows `retention: n/a` - VM does not expose
  `/api/v1/status/runtimeinfo`.
- `cardinality top` derives `numSeries` by summing
  `seriesCountByMetricName` (VM does not return `headStats`).
- Cortex/Mimir-style multi-tenancy headers (`X-Scope-OrgID`) are not
  supported; URL-prefix-based tenant routing through `vmauth` works.

## Alert hygiene + reports

remetric inspects the `ALERTS` series via `query_range` to flag alerts that
either never fire or fire continuously (broken thresholds, alert noise).

```bash
# Alerts that did not fire in the last 7 days (default lookback)
remetric alerts unused \
  --prometheus http://localhost:9090

# Alerts that fire >=95% of the lookback window
remetric alerts always-firing \
  --prometheus http://localhost:9090 \
  --lookback 24h \
  --step 5m
```

Tune the sampling window with `--lookback` (default `168h`) and `--step`
(default `1h`). For VictoriaMetrics, point `--vmalert` at the vmalert API.

### Unified report

`remetric report` runs every analyzer and emits a single document in
terminal, JSON, HTML, or Markdown format.

```bash
# Self-contained HTML report (opens in any browser, mobile-friendly)
remetric report --prometheus http://localhost:9090 \
  --format html --out report.html

# Markdown for PR comments / inboxes
remetric report --prometheus http://localhost:9090 \
  --format markdown > report.md
```

Formats: `terminal` (default), `json`, `html`, `markdown`. Use `--out FILE`
to write to a file, or `-` (the default) for stdout. The global `--output`
flag is ignored by `report` - use `--format` instead.

## Commands

| Command                            | What it does                                                |
|------------------------------------|-------------------------------------------------------------|
| `remetric doctor`                  | Connectivity + version + permission self-check              |
| `remetric cardinality top`         | List the worst-offending high-cardinality metric/label pairs|
| `remetric cardinality labels`      | Per-metric label inventory (unique counts + sample values)  |
| `remetric cardinality suspicious`  | Flag labels matching unbounded-identifier patterns          |
| `remetric metrics unused`          | Ingested ∖ used metrics (needs Grafana for dashboard coverage)|
| `remetric alerts unused`           | Alerts that never fired in the lookback window              |
| `remetric alerts always-firing`    | Alerts firing >=95% of the lookback window                  |
| `remetric dashboards broken`       | Flag dashboards whose panels reference missing metrics      |
| `remetric report`                  | Run every analyzer, render terminal/json/html/markdown      |
| `remetric scan`                    | Run every available analyzer, emit a unified Report         |

Global flags (subset; see `--help` for the full list):

- `--prometheus URL` - Prometheus base URL. Env: `REMETRIC_PROMETHEUS_URL`.
- `--prom-token TOK` - Bearer token. Env: `REMETRIC_PROMETHEUS_TOKEN`.
- `--grafana URL` - Grafana base URL. Env: `REMETRIC_GRAFANA_URL`.
- `--grafana-token TOK` - Grafana service-account API key. Env: `REMETRIC_GRAFANA_TOKEN`.
- `--grafana-basic-auth USER:PASS` - Basic auth for Grafana.
- `--grafana-tls-skip-verify` - Skip TLS verification for Grafana.
- `--backend {auto|prometheus|victoria}` - backend dialect. Env: `REMETRIC_BACKEND`.
- `--vmalert URL` - vmalert base URL for /api/v1/rules. Env: `REMETRIC_VMALERT_URL`.
- `--vmalert-token TOK` - Bearer for vmalert (inherits from --prom-token if unset). Env: `REMETRIC_VMALERT_TOKEN`.
- `--vmalert-basic-auth USER:PASS` - Basic auth for vmalert (inherits from --prom-basic-auth if unset).
- `--vmalert-tls-skip-verify` - Skip TLS verify for vmalert.
- `--prom-basic-auth USER:PASS` - Basic auth.
- `--prom-max-in-flight N` - Concurrency cap (default 5).
- `--output FORMAT` - `terminal` (default) or `json`.
- `--fail-on SEV` - Exit 3 if any finding is at or above this severity. Env: `REMETRIC_FAIL_ON`. Default `none`.
- `--no-color` - Disable colored output (`NO_COLOR` env also respected).
- `--verbose` - Debug-level slog logging on stderr.

## Documentation

Full reference at [remetric.dev](https://remetric.dev/) - one page per finding
class with detection rules, fix snippets, and false-positive notes.

## CI integration

Pair any analyzer command with `--fail-on=critical` to fail the build when a
finding at or above the chosen severity is present. Default behaviour
(`--fail-on=none`) preserves zero-exit regardless of findings.

```bash
# Fail the build if any critical-severity finding is present
remetric scan --prometheus http://localhost:9090 --fail-on=critical
```

Exit codes:

| Code | Meaning |
|------|---------|
| 0    | Clean exit (no findings ≥ threshold, or `--fail-on=none`). |
| 1    | Runtime or analyzer error. |
| 2    | Flag / usage error. |
| 3    | Findings at or above `--fail-on` threshold. |

## Ignoring findings

Suppress findings that are known noise or out of scope. Patterns are
**anchored full-match** regexes: `foo_.*` matches `foo_bar` but not
`xfoo_bar`. Empty / whitespace-only patterns are silently ignored.

Four target fields, each with its own flag (repeatable):

| Flag | Drops findings whose ... |
|------|--------------------------|
| `--ignore-metric REGEX`    | metric name matches |
| `--ignore-label REGEX`     | evidence label matches |
| `--ignore-alert REGEX`     | alert name matches |
| `--ignore-dashboard REGEX` | dashboard title matches |

```bash
# Repeatable flag
remetric scan \
  --prometheus http://localhost:9090 \
  --ignore-metric='node_.*' \
  --ignore-metric='go_.*' \
  --ignore-alert='HighMemoryUsage'

# Environment (comma-separated lists)
REMETRIC_IGNORE_METRIC='node_.*,go_.*' \
REMETRIC_IGNORE_ALERT='HighMemoryUsage' \
  remetric scan --prometheus http://localhost:9090

# YAML at ~/.config/remetric/.remetric.yaml or ./.remetric.yaml
# ignore:
#   metric: ["node_.*", "go_.*"]
#   label:  ["pod"]
#   alert:  ["HighMemoryUsage"]
```

The dropped count surfaces in every output format. Filter runs BEFORE
`--fail-on`, so an ignored critical finding does not raise exit code 3.

## Supported versions

| Component | Minimum | Tested | Notes |
|-----------|---------|--------|-------|
| Prometheus | 2.30 | 2.51.x, 2.53.x | TSDB stats API (`/api/v1/status/tsdb`) is the floor. Prometheus 3.x untested - file an issue if you hit something. |
| VictoriaMetrics | v1.93 | v1.108.x | Single-binary + cluster (via `vmauth`). `vmalert` required for alert + recording-rule coverage; pass `--vmalert`. |
| Grafana | 9.0 | 10.4.x, 11.x | Service-account API keys preferred (`--grafana-token`); basic auth supported. |
| Go (build) | 1.26 | 1.26.3 | Only needed if building from source; releases ship static binaries. |
| OS / arch | - | linux+amd64, linux+arm64, darwin+amd64, darwin+arm64, windows+amd64 | Static binary, no glibc dependency. |
| Docker | - | 24+ | For demo / e2e stacks via `docker compose`. |

Multi-tenant Cortex / Mimir / Thanos: URL-prefix tenant routing through
`vmauth` works; `X-Scope-OrgID` header style is not yet supported.

## FAQ

**Does remetric modify my Prometheus or Grafana?**
No. Remetric is strictly read-only. It calls `GET` against the Prometheus HTTP API and Grafana's `/api/search` + `/api/dashboards`. Nothing is created, updated, or deleted.

**What does each severity mean?**
- `critical` - clear, large-impact problem (broken always-firing alert; metric with millions of series concentrated in one unbounded label).
- `high` - significant cardinality offender or unused metric responsible for >5% of total series.
- `medium` - notable but bounded (suspicious label pattern, never-fired alert).
- `low` - informational; below the default `--min-severity=medium` cutoff, surfaced via `--min-severity=low`.

Severity is computed per-analyzer from observed series counts, uniqueness ratios, and lookback windows. See `internal/scoring/` for the exact rules.

**How accurate is the series-reduction estimate?**
It's an upper bound (`estimation_method: "labeldrop_upper_bound"`). The number assumes the offending label is fully dropped; in practice, partial relabel rules will reduce less. Treat it as "this much waste *could* go away if you fully suppress this label".

**Does `--ignore` interact with `--fail-on`?**
Ignored findings are dropped BEFORE the `--fail-on` gate. An ignored critical finding does not raise exit code 3. This is intentional: ignored == "known and accepted".

**Why do I need `--vmalert` for VictoriaMetrics?**
VictoriaMetrics serves `/api/v1/rules` from `vmalert`, not from `vmselect`. Without `--vmalert`, the alert-hygiene analyzer and unused-metric analyzer can't see rule definitions and will warn `rules unavailable`. Point `--vmalert` at the vmalert HTTP listener (default `:8880`).

**`scan` vs `report` vs `cardinality top` - which one do I run?**
- `cardinality top` (and other focused commands) - drill into one analyzer's output. Use for investigation.
- `scan` - run every analyzer, emit a JSON `findings.Report`. Use in CI / scripts (with `--fail-on`).
- `report` - same coverage as `scan` but renders to `terminal` (default), `json`, `html`, or `markdown` via `--format`. Use to share a snapshot.

**How do I run remetric in CI?**
Pair any command with `--fail-on=critical` (or stricter) so a regression breaks the build:

```bash
remetric scan --prometheus https://prom.internal --fail-on=critical --output json > scan.json
```

Exit codes: `0` clean, `1` runtime error, `2` flag/usage error, `3` findings at or above threshold. See `## CI integration`.

**Can I silence known-noisy metrics or alerts?**
Yes, with `--ignore-metric`, `--ignore-label`, `--ignore-alert` (anchored regex, repeatable). Patterns can also come from `REMETRIC_IGNORE_*` env vars or `.remetric.yaml`. See `## Ignoring findings`.

`--ignore` applies to commands that produce `Finding`s: `scan`, `report`, `cardinality top`, `cardinality suspicious`, `alerts unused`, `alerts always-firing`, `metrics unused`. It does NOT apply to `cardinality labels` (which emits a label inventory, not findings) or `doctor` (connectivity self-check).

**What about Loki / multi-tenant Cortex / Mimir?**
Post-v0.1 roadmap. Today, single-tenant Prometheus + VictoriaMetrics. Tenant routing via `vmauth` URL prefixes works; `X-Scope-OrgID` is not yet wired.

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
