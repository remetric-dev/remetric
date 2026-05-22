# Never-firing alert

**Class:** `never-firing-alert`
**Severity (typical):** Medium
**Category:** `alert_hygiene`
**Finding ID prefix:** `alert_hygiene/never_fired`

## What it means

A *never-firing alert* is an alerting rule that has not produced a single
firing sample over the lookback window. It is loaded into Prometheus's rule
engine, evaluated on every interval, and consumes its share of evaluation
budget - but nothing it watches has ever crossed the threshold.

Sometimes that's fine: a rule that fires only during real outages should stay
quiet between them. But on a long lookback (the default is a full week), a
rule that has *never* tripped is suspicious. Common causes are: a typo in the
PromQL expression that always returns empty; a threshold tuned for a metric
that has since been renamed; a rule someone copy-pasted from a template and
forgot to wire to a real signal; or a rule for a code path that has been
removed entirely. Each of these is an alert you think is protecting you when
it isn't.

## How remetric detects it

The alert-hygiene analyzer (`internal/analyzers/alerthygiene`) does the
following:

1. Fetch the rule tree from `/api/v1/rules` (preferring `--vmalert` when set).
2. Flatten to alerting rules only (recording rules are ignored).
3. For each alert, run `query_range` on `ALERTS{alertname="<name>"}` over the
   configured lookback (default 168h) with the configured step (default 1h).
4. Sum samples where `alertstate="firing"`.

Defaults come from the analyzer's `Config` and the `--lookback` / `--step`
flags. The query is bounded by the same `--prom-max-in-flight` semaphore as
every other Prometheus call.

If the sum is zero, the alert is classified as *never-fired* and a finding is
emitted at **Medium** severity. (An alert with some firing samples but below
95% of steps is classified as `none` and produces no finding; an alert at or
above 95% becomes an [always-firing](always-firing-alert.md) finding instead.)

The evidence string includes the alertname, the rule's source file, and the
group name, so you can navigate straight to the YAML.

## How to fix it

The analyzer emits a rule-change snippet that points at the offending rule:

```yaml
# In <file>, group <group>: consider removing or relaxing this alert
- alert: <name>
  expr: <original expression>
```

Three possible fixes, in order of how often each is right:

1. **Delete the rule.** If you can't reconstruct why it exists, it isn't
   protecting anything. Remove it from the rules file and reload.
2. **Relax the threshold.** If the rule used to fire but never does now (e.g.
   you scaled up and your "high memory" line is no longer reachable), recompute
   the threshold against current baseline.
3. **Fix the PromQL.** If the expression references a metric that doesn't
   exist, the `query_range` for `ALERTS{...}` will be empty. Inspect the
   expression - `count(<expr>) by ()` over the same lookback will tell you
   whether the rule's input series even exists.

## False positives

You may want to ignore this class when:

- The alert is for a rare disaster signal (datacenter loss, backup integrity
  check) that legitimately should never fire in steady state.
- The lookback is too long for a recently deployed rule. Re-run with
  `--lookback 24h` and re-evaluate.

Suppress via `--ignore-alert <regex>`. See the
[README's "Ignoring findings" section](https://github.com/remetric-dev/remetric#ignoring-findings).

## Related

- [Always-firing alert](always-firing-alert.md) - the opposite failure mode:
  an alert that fires continuously and therefore signals nothing.
