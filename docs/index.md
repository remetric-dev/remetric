# remetric

> Re-metric your stack - find waste in Prometheus, Grafana & Loki.

remetric is a read-only doctor for self-hosted observability stacks. Point it at
a Prometheus server (optionally a Grafana too) and it returns a ranked,
actionable list of cardinality problems, unused metrics, label-pattern
anti-patterns, and alert-hygiene findings - each with a paste-ready fix.

## What it finds

- [Hot labels](findings/hot-label.md) - cardinality explosions driven by a single label.
- [Unused metrics](findings/unused-metric.md) - metrics scraped but never referenced by any dashboard or rule.
- [Overly granular label patterns](findings/label-pattern-overly-granular.md) - labels that look like unbounded identifiers.
- [Never-firing alerts](findings/never-firing-alert.md) - rules whose threshold has not triggered over the lookback window.
- [Always-firing alerts](findings/always-firing-alert.md) - rules whose threshold is permanently met (likely broken).

## Get started

[Read the getting-started guide](getting-started.md), or jump straight to the
[GitHub repository](https://github.com/remetric-dev/remetric).
