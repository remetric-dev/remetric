# Label pattern overly granular

**Class:** `label-pattern-overly-granular`
**Severity (typical):** Low / Medium (High / Critical at very high uniqueness)
**Category:** `label_patterns`
**Finding ID prefix:** `label-<label>`

## What it means

Some label *names* tell you, before you even look at their values, that they
will explode in cardinality. A label called `trace_id` will not contain "prod"
and "staging" - it will contain a hash per request. The same is true for
`user_id`, `session_token`, free-form `path` / `url` / `email`, or anything
ending in `_uuid`. The label-pattern analyzer flags labels by *shape of name*,
not (only) by current cardinality, so you catch the explosion before it gets
critical.

This class is a leading indicator for [hot-label](hot-label.md) findings.
Catching a `request_id` label at 500 unique values and labeldropping it now
is cheaper than discovering it at five million values during an incident.

## How remetric detects it

The label-pattern analyzer (`internal/analyzers/labelpattern`) walks
`/api/v1/status/tsdb`'s `labelValueCountByLabelName` and applies, in order:

1. **Bounded allowlist** - labels named `cluster`, `environment`, `region`,
   `namespace`, `job`, or `instance` are skipped regardless of cardinality.
   These are operational labels that dashboards depend on.
2. **Suspicious-name patterns** (case-insensitive regex):
   - `.*(uuid|guid).*`
   - `.*_id$`
   - `^(path|url|uri|endpoint)$`
   - `.*(trace|span)_id.*`
   - `^(session|request)_.*`
   - `^(email|user(name)?)$`
3. **Uniqueness magnitude** via `scoring.LabelPatternSeverity`:

   | Tier | Trigger |
   |---|---|
   | Critical | uniqueValues > 5,000 |
   | High | uniqueValues > 1,000 |
   | Medium | uniqueValues > 100 |
   | Low | otherwise |

4. **Sample-value sanity check** - the analyzer fetches a sample of values via
   `/api/v1/label/<l>/values`. If the samples look bounded (e.g. enum-like
   tokens, not random hex), severity is downgraded one tier and the evidence
   carries `(sampled values look bounded - possible false positive)`.
5. **Affected-metrics tally** - the analyzer also calls
   `/api/v1/series` (via `MetricNamesWithLabel`) to find every metric that
   carries the label, and sums their head series counts into
   `Evidence.SeriesCount` and an `Impact` estimate.

## How to fix it

The analyzer emits a `metric_relabel_configs` snippet that labeldrops the
label across the affected metrics, with the metric list inlined as comments:

```yaml
metric_relabel_configs:
  - regex: '<label>'
    action: labeldrop
# Affected metrics (N):
#   - metric_a_total
#   - metric_b_seconds_bucket
```

Drop this into each `scrape_config` whose metrics appear in the affected list.
Because the rule is per-job, you may need to repeat it across jobs that all
expose the same offender. New scrapes will land without the label and the
series for the affected metrics will collapse onto the remaining label
combinations.

## False positives

You may want to ignore this class when:

- A `*_id` label is genuinely bounded (e.g. `tenant_id` in a four-tenant
  deployment). The sample-value sanity check downgrades severity but the
  finding still appears.
- An exporter you don't control emits a `path` or `url` label and you want
  to keep it on a low-volume metric for diagnostics.

Suppress via `--ignore-label <regex>` or `--ignore-metric <regex>`. See the
[README's "Ignoring findings" section](https://github.com/remetric-dev/remetric#ignoring-findings).

## Related

- [Hot label](hot-label.md) - the per-metric view of the same problem; one
  metric's worst label drives a hot-label finding even if the label's name
  doesn't match a suspicious pattern.
- [Unused metric](unused-metric.md) - if the metric carrying the label is
  unused, drop the whole metric.
