# Unused metric

**Class:** `unused-metric`
**Severity (typical):** Low / Medium (Critical / High at very large series counts)
**Category:** `unused_metrics`
**Finding ID prefix:** `unused-<metric>`

## What it means

An *unused metric* is a metric that Prometheus ingests on every scrape but
that nothing in your observability stack ever reads back. No Grafana dashboard
plots it, no alert rule references it, no recording rule rolls it up. Its
series sit in the head block, get flushed to the WAL, compact into chunks, and
eventually age out of retention - having served exactly zero queries.

Unused metrics are quiet money. They don't break anything. They don't trip an
on-call alert. They just inflate your bytes-on-disk, your scrape duration, and
your TSDB index size in the background. The fix is almost always cheap (one
`metric_relabel_configs` block), but the analyzer can't be 100% certain a
metric is unused - ad-hoc queries leave no audit trail - so this class
deliberately scores conservatively.

## How remetric detects it

The unused-metric analyzer (`internal/analyzers/unusedmetrics`) builds two
sets:

- **Ingested** - every `__name__` value returned by
  `/api/v1/label/__name__/values`.
- **Used** - the union of metric names extracted from
  - every panel query across every Grafana dashboard found via `/api/search`
    and `/api/dashboards` (when `--grafana` is provided), parsed by
    `promqlx.ExtractFromQuery`;
  - every alerting and recording rule expression in `/api/v1/rules`, parsed
    the same way;
  - the output name of every recording rule (the rule's own metric).

`ingested \ used` becomes a finding per metric. Series counts come from
`/api/v1/status/tsdb` and feed `scoring.UnusedMetricSeverity`:

| Tier | Trigger |
|---|---|
| Critical | series > 100,000 |
| High | series > 25,000 |
| Medium | series > 5,000 |
| Low | otherwise |

These thresholds are intentionally lower than the cardinality analyzer's
because false-positive risk is higher. Impact is a *full drop*
(`estimation_method: "full_drop"`, `percentage: 100`) - if you stop ingesting
the metric, the series count goes to zero.

On VictoriaMetrics, `/api/v1/rules` is served by `vmalert`. Without
`--vmalert`, recording rules are invisible and the analyzer warns
`rules unavailable: VictoriaMetrics detected without --vmalert URL`. Some
metrics will be reported as unused that are actually consumed by a recording
rule - treat findings as suspect until you provide `--vmalert`.

## How to fix it

The analyzer emits a `metric_relabel_configs` snippet that drops the metric at
ingest time:

```yaml
metric_relabel_configs:
  - source_labels: [__name__]
    regex: '<metric>'
    action: drop
```

Apply it to the scrape job that exposes the metric. Series already in the head
will age out over your retention window; no new series for that metric will
land.

## False positives

You may want to ignore this class when:

- The metric is only used in ad-hoc PromQL (Grafana Explore queries,
  troubleshooting CLI). The analyzer has no signal for that traffic.
- Grafana coverage is partial (`--grafana` not provided, or a dashboard folder
  the bearer token can't read).
- VictoriaMetrics without `--vmalert` - the analyzer warns explicitly.

Suppress via `--ignore-metric <regex>`. See the
[README's "Ignoring findings" section](https://github.com/remetric-dev/remetric#ignoring-findings).

## Related

- [Hot label](hot-label.md) - if the metric is heavily used but bloated by one
  label, fix the label rather than dropping the metric.
- [Dashboard sprawl](dashboard-sprawl.md) - if the metric is referenced only by
  dashboards nobody looks at, its "used" status may itself be a lie.
