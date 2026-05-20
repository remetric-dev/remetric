# remetric

Re-metric your stack — find waste in Prometheus, Grafana & Loki.

`remetric` is a read-only doctor for self-hosted Prometheus stacks. Point it at a
Prometheus server and it prints a ranked, actionable list of cardinality
problems with suggested `metric_relabel_configs` fixes.

> Status: **alpha (Phase 1 of v0.1)** — only the cardinality analyzer is wired
> up. Grafana integration, alert hygiene, unused-metric detection, and HTML/JSON
> reports come in later phases.

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

## Commands

| Command                     | What it does                                                |
|-----------------------------|-------------------------------------------------------------|
| `remetric doctor`           | Connectivity + version + permission self-check              |
| `remetric cardinality top`  | List the worst-offending high-cardinality metric/label pairs|

Global flags (subset; see `--help` for the full list):

- `--prometheus URL` — Prometheus base URL. Env: `REMETRIC_PROMETHEUS_URL`.
- `--prom-token TOK` — Bearer token. Env: `REMETRIC_PROMETHEUS_TOKEN`.
- `--prom-basic-auth USER:PASS` — Basic auth.
- `--prom-max-in-flight N` — Concurrency cap (default 5).
- `--no-color` — Disable colored output (`NO_COLOR` env also respected).
- `--verbose` — Debug-level slog logging on stderr.

## What does Phase 1 *not* do yet?

- No JSON / HTML / Markdown output.
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
