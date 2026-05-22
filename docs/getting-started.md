# Getting started

A 60-second tour from install to your first scan.

## Install

The one-line install script drops the latest release into `$HOME/.local/bin`:

```bash
curl -sSL https://remetric.dev/install.sh | sh
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

```text
▸ cardinality... done (118ms)
▸ labelpattern... done (8ms)
▸ unusedmetrics... done (13ms)
▸ alerthygiene... done (7ms)
┌──────────┬────────────────────┬──────────┬────────┬────────┬────────────────┐
│ SEVERITY │ METRIC             │ LABEL    │ SERIES │ UNIQUE │ EST. REDUCTION │
├──────────┼────────────────────┼──────────┼────────┼────────┼────────────────┤
│ CRITICAL │ app_requests_total │ trace_id │ 500    │ 500    │ ~499           │
│ MEDIUM   │                    │ user_id  │ 500    │ 500    │ ~499           │
│ MEDIUM   │                    │ trace_id │ 500    │ 500    │ ~499           │
│ MEDIUM   │                    │          │ 0      │ 0      │ ~0             │
└──────────┴────────────────────┴──────────┴────────┴────────┴────────────────┘

[CRITICAL] app_requests_total  ·  trace_id has 500 unique values
Sample: trace-001f71cef44a4aca, trace-006ce2eaa75c9f65, trace-007fff8ea657221b, trace-0102b61d60431cf5, trace-0116b64ca9b86791
Estimated reduction (upper bound): 499 series

Suggested fix (Prometheus scrape config):
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: "app_requests_total"
        action: keep
      - regex: "trace_id"
        action: labeldrop
Reference: https://remetric.dev/findings/hot-label
```

## Reading the report

The output has three layers:

1. **Phase tracker** at the top - each analyzer logs when it starts and how
   long it took. Useful for spotting a slow analyzer or a hung Prometheus.
2. **Severity table** - at-a-glance ranking. Columns:
    - `SEVERITY` - `CRITICAL` / `HIGH` / `MEDIUM` / `LOW`, computed from
      observed series counts, uniqueness ratios, and lookback windows.
    - `METRIC` / `LABEL` - the entity that triggered the finding.
    - `SERIES` - total series in the metric.
    - `UNIQUE` - unique values seen on the offending label.
    - `EST. REDUCTION` - upper-bound series saved if you apply the fix.
3. **Per-finding detail block** - for `CRITICAL` and `HIGH` rows the table is
   followed by sample values, the exact reduction, a paste-ready fix snippet
   for `prometheus.yml`, and a `Reference:` link to the documentation page
   for that finding class.

The same data is available in JSON via `--output json` (for CI), as a
self-contained HTML report (`remetric report --format html --out report.html`),
or as Markdown for PR comments (`--format markdown`). JSON and Markdown
include the `documentation_url` for every finding so you can pipe scan results
into tickets, Slack, or a dashboard with one-click links into this site.

## Next steps

- Browse the [finding catalog](findings/index.md) to see every class, what it
  detects, and how to fix it.
- Wire `remetric scan --fail-on=critical` into CI to fail builds on regressions.
- Suppress known noise with `--ignore-metric`, `--ignore-label`, or
  `--ignore-alert` (anchored regex, repeatable).
