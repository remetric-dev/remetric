# Always-firing alert

**Class:** `always-firing-alert`
**Severity (typical):** Critical
**Category:** `alert_hygiene`
**Finding ID prefix:** `alert_hygiene/always_firing`

## What it means

An *always-firing alert* is an alerting rule whose threshold is met across
essentially the entire lookback window. The on-call gets a page, the page
clears, the on-call gets a page, the page clears - on a loop, because the
underlying signal never actually goes back below the line.

This is alert noise at its worst. Always-firing alerts train on-call humans
to ignore a specific alertname; that habit then bleeds into ignoring nearby
alerts that *do* mean something. They also corrupt incident timelines: a
postmortem will see the alert firing continuously through the incident and
struggle to identify when the real signal started.

The cause is almost always one of: a threshold that was set against a
load-test environment and never re-tuned for production; a metric whose units
or normalization changed since the rule was written; or a rule that was
flipped during refactoring (`>` should have become `<`).

## How remetric detects it

The alert-hygiene analyzer (`internal/analyzers/alerthygiene`) runs the same
pipeline as the [never-firing](never-firing-alert.md) class:

1. Fetch alerting rules from `/api/v1/rules` (preferring `--vmalert` when set).
2. For each rule, query `ALERTS{alertname="<name>"}` via `query_range` over
   the lookback (default 168h, step 1h).
3. Sum samples where `alertstate="firing"`.

The classifier compares the firing-sample sum to total steps in the window
(ceil(lookback / step)):

| Tier | Trigger |
|---|---|
| Critical (always-firing) | firing ratio >= 0.95 |
| (none) | 0 < ratio < 0.95 |
| Medium (never-firing) | ratio == 0 |

The 0.95 threshold is the `alwaysFiringThreshold` constant in the analyzer.
The finding's title includes the exact ratio and step count
(`Alert X firing 97.3% of window (163/168 steps)`) so you can see how close
to permanent the rule is.

## How to fix it

The analyzer emits a rule-change snippet that points at the offending rule:

```yaml
# In <file>, group <group>: review threshold - alert is permanently firing
- alert: <name>
  expr: <original expression>
```

Walk the expression in a graph view (`/graph?g0.expr=<expr>&g0.range_input=1w`)
to see what the input series actually looks like. The fix is usually one of:

1. **Re-tune the threshold.** Compute current production p99 / p95 / p50 over
   a representative window and pick a threshold that fires on the tail you
   actually care about, not on baseline.
2. **Flip the comparison.** If `up{job="x"} > 0` is firing 100% of the time,
   you almost certainly meant `up{job="x"} == 0`.
3. **Add a `for:` clause.** A rule with no `for:` fires on every sample over
   the line. Adding `for: 5m` keeps brief spikes from generating a page (and
   surfaces real sustained breaches).
4. **Delete the rule** if the signal it was meant to catch no longer matters.

## False positives

You may want to ignore this class when:

- An alert is intentionally always-firing (e.g. a heartbeat-style "the rule
  engine is alive" alert that you route to silence). Move these to silences
  rather than alert rules, but if you can't, ignore via `--ignore-alert`.
- A rule was just deployed during an active incident and the lookback hasn't
  caught up. Re-run with a shorter `--lookback` to confirm.

Suppress via `--ignore-alert <regex>`. See the
[README's "Ignoring findings" section](https://github.com/remetric-dev/remetric#ignoring-findings).

## Related

- [Never-firing alert](never-firing-alert.md) - the opposite failure mode:
  an alert loaded but never tripped.
