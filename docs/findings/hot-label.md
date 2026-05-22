# Hot label

**Class:** `hot-label`
**Severity (typical):** Critical / High / Medium
**Category:** `cardinality`
**Finding ID prefix:** `card-<metric>-<label>`

## What it means

A *hot label* is a single label on a single metric that drives the metric's
series count into the danger zone. Cardinality in Prometheus is multiplicative:
each unique combination of label values across `(__name__, label_1, ..., label_n)`
becomes its own time series, with its own chunk in the TSDB head block and its
own slot in every query plan. One label with thousands of distinct values
(`user_id`, `request_id`, a free-form `path`) can balloon a single metric to
millions of series.

The cost is concrete. Head-block RSS scales with active series. `query_range`
fanout time scales with series matched. WAL replay on restart scales with
series. A hot label is rarely the only cause of stack-wide cardinality issues,
but it is usually the most actionable: dropping one label on one metric can
shed a quarter of a tenant's series in a single deploy.

## How remetric detects it

The cardinality analyzer (`internal/analyzers/cardinality`) calls
`/api/v1/status/tsdb` to get the top metrics by `seriesCountByMetricName`.
For each metric it lists labels via `/api/v1/labels` (matcher `{__name__="<m>"}`)
and queries `/api/v1/label/<l>/values` per label to get the unique-value count.
The label with the highest uniqueness is the *worst label*; ties break
lexicographically (smallest label name wins) for deterministic output.

Severity is computed against the head's total series via
`scoring.CardinalitySeverity`:

| Tier | Trigger |
|---|---|
| Critical | series > 300,000 **or** > 15% of head |
| High | series > 100,000 **or** > 5% of head |
| Medium | series > 25,000 |
| Low | below all of the above (suppressed - not emitted) |

Boundaries are strict greater-than (300,000 exact stays at High). The impact
field reports an *upper bound* series reduction assuming the offending label is
fully dropped (`estimation_method: "labeldrop_upper_bound"`).

## How to fix it

The analyzer emits a `metric_relabel_configs` snippet keyed on the metric and
labeldropping the offender:

```yaml
metric_relabel_configs:
  - source_labels: [__name__]
    regex: "<metric>"
    action: keep
  - regex: "<label>"
    action: labeldrop
```

Drop this into the `scrape_config` for the job that exposes the metric. The
`keep` action scopes the labeldrop to that one metric so other metrics that
legitimately need the label are untouched. After a reload, the TSDB will keep
old series until they age out of the retention window; new series for the
metric will collapse to the lower cardinality immediately.

## False positives

You may want to suppress this class when:

- The label is operationally required (e.g. a `tenant` label on a multi-tenant
  exporter). Move it to a `bounded_labels` allowlist in your deployment
  conventions rather than silencing remetric.
- A metric is high-cardinality by design (a histogram backing a SLO query that
  needs full resolution).

Suppress via `--ignore-metric <regex>` or `--ignore-label <regex>`. See the
[README's "Ignoring findings" section](https://github.com/remetric-dev/remetric#ignoring-findings)
for the regex anchoring rules and YAML/env equivalents.

## Related

- [Label pattern overly granular](label-pattern-overly-granular.md) - flags
  labels by *name shape* (e.g. anything ending in `_id`) even before they grow
  to hot-label severity.
- [Unused metric](unused-metric.md) - if the metric itself is unused, drop the
  whole metric instead of just the label.
