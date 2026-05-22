# Getting started

A 60-second tour from install to your first scan.

## Install

The one-line install script drops the latest release into `$HOME/.local/bin`:

```bash
curl -sSL https://raw.githubusercontent.com/remetric-dev/remetric/main/install.sh | sh
```

Override the destination with `REMETRIC_INSTALL_DIR=/usr/local/bin sh install.sh`,
or pin a specific version via `REMETRIC_VERSION=v0.1.0 sh install.sh`.

Prefer containers? A multi-arch image is published as
`ghcr.io/remetric-dev/remetric:latest` (`linux/amd64`, `linux/arm64`).

```bash
docker run --rm \
  ghcr.io/remetric-dev/remetric:latest \
  scan --prometheus http://host.docker.internal:9090
```

Static binaries and a `checksums.txt` are attached to every
[GitHub release](https://github.com/remetric-dev/remetric/releases).

## First scan

Run every analyzer in one shot. `scan` only needs a Prometheus URL to start
producing findings; add `--grafana` for richer unused-metric coverage.

```bash
remetric scan --prometheus http://localhost:9090
```

The terminal output looks like this:

```
SEVERITY  CLASS                 METRIC                 LABEL          FIX
critical  hot-label             http_requests_total    user_id        labeldrop user_id
high      unused-metric         go_memstats_alloc_old  -              drop_metric
medium    never-firing-alert    HighRequestLatencyP99  -              relax threshold
medium    label-pattern-overly  prometheus_http_*      trace_id       labeldrop trace_id
```

## Reading the report

Every finding has the same shape:

- **Severity** - `critical`, `high`, `medium`, `low`. Computed from observed
  series counts, uniqueness ratios, and lookback windows.
- **Class** - a stable slug (e.g. `hot-label`) that identifies the type of
  waste, independent of which entity triggered it. Each class has a dedicated
  documentation page reachable from the report's `documentation_url`.
- **Metric / Label / Alert** - the entity that triggered the finding.
- **Fix** - a paste-ready `metric_relabel_configs` or rule-change snippet you
  drop into your Prometheus config to make the waste go away.

The same data is available in JSON via `--output json` (for CI), as a
self-contained HTML report (`remetric report --format html --out report.html`),
or as Markdown for PR comments (`--format markdown`).

## Next steps

- Browse the [finding catalog](findings/index.md) to see every class, what it
  detects, and how to fix it.
- Wire `remetric scan --fail-on=critical` into CI to fail builds on regressions.
- Suppress known noise with `--ignore-metric`, `--ignore-label`, or
  `--ignore-alert` (anchored regex, repeatable).
