# Broken panel

A Grafana dashboard panel queries a Prometheus metric that does not exist.

**Class:** `broken-panel`
**Severity:** Medium
**Category:** `dashboard_hygiene`
**Detected by:** `remetric dashboards broken`, `remetric scan`

## What it means

The panel's PromQL target references a metric name that is not in
Prometheus head series and is not declared as the output of any
recording rule. The panel will render as empty (no data) or as a
flat line, and the user will see a silent gap rather than the
expected signal.

## Why it matters

Empty dashboard tiles are dangerous because they look the same as
a system that is fine. An on-call engineer scanning a dashboard
for problems can miss a real issue because the relevant tile shows
"no data" instead of the metric they expect. A scan that surfaces
every (dashboard, missing-metric) pair lets you either restore the
metric or remove the dead query.

## How remetric detects it

1. Build the set of "known metrics":
    - Every metric name returned by `/api/v1/label/__name__/values`.
    - Every recording-rule output name returned by `/api/v1/rules`
      (a freshly-added recording rule counts as known even before
      it emits its first sample).
2. For each Grafana dashboard, parse every panel's Prometheus
   target with the PromQL parser and extract referenced metric
   names.
3. Any extracted name not in the known set is "missing". Findings
   are grouped by `(dashboard, missing-metric)` so 50 panels in one
   dashboard referencing the same removed metric collapse into a
   single finding listing the affected panels.

## Known false positives

- **Intermittent metrics.** A metric that is only present during
  scheduled cron-job runs may appear missing during a scan that
  runs between executions. Suppress with `--ignore-metric <regex>`.
- **Freshly-rotated retention.** If a metric is in long-term storage
  but no longer in head series, it counts as missing. Most users
  do not hit this because head-series retention is usually 15 days
  or more.
- **VictoriaMetrics without `--vmalert`.** Recording rules live in
  the `vmalert` process, not in `vmselect`. When `--vmalert` is not
  provided, recording outputs are not visible to remetric and
  any panel querying a recording-rule output is reported as broken.
  The analyzer surfaces this case as a warning rather than failing.

## How to fix

Pick one of:

1. **Restore the metric.** Re-enable the scrape job, fix the
   exporter, or add back the recording rule that emits the
   metric.
2. **Remove the dead query.** Edit the dashboard in Grafana,
   delete the panel or rewrite its query to use a metric that
   still exists.
3. **Suppress.** If the dashboard is known-stale and you cannot
   delete it yet, use `--ignore-dashboard "Legacy.*"` (anchored
   regex against the dashboard title).

## Sample JSON

```json
{
  "id": "broken-panel:abc123:node_disk_io_now",
  "severity": "medium",
  "category": "dashboard_hygiene",
  "class": "broken-panel",
  "title": "dashboard \"Frontend SLOs\" references missing metric \"node_disk_io_now\"",
  "metric": "node_disk_io_now",
  "dashboard": "Frontend SLOs",
  "evidence": {
    "description": "2 panel(s) in dashboard \"Frontend SLOs\" query \"node_disk_io_now\" which is not present in head series or recording-rule outputs",
    "sample_values": ["Disk I/O - last 5m", "Disk I/O - last 1h"]
  },
  "fix": {
    "type": "edit_dashboard",
    "config": "Edit dashboard \"Frontend SLOs\" (https://grafana.example.com/d/abc123/frontend-slos)\nand either:\n  1. Restore metric \"node_disk_io_now\" (...)\n  2. Remove/replace the broken queries in panel(s):\n     - Disk I/O - last 5m\n     - Disk I/O - last 1h\nReference: https://remetric.dev/findings/broken-panel"
  },
  "impact": {
    "series_reduction": 0,
    "percentage": 0,
    "estimation_method": "broken_panel"
  },
  "documentation_url": "https://remetric.dev/findings/broken-panel"
}
```
