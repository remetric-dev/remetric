# Dashboard sprawl

**Class:** `dashboard-sprawl`
**Category:** `dashboard_sprawl`

!!! info "Not yet implemented"
    The `DashboardSprawlAnalyzer` is on the v0.1 launch path but not yet
    shipped. The Class slug is reserved.

## What it will detect

When implemented, this class will surface:

- **Untouched dashboards** - dashboards that haven't been viewed in over N days
  (last-viewed timestamp via the Grafana API).
- **Broken panels** - dashboard panels whose queries reference metrics that no
  longer exist (or never existed) in Prometheus.

## Status

See the [backlog](https://github.com/remetric-dev/remetric/issues) for current
implementation status.
