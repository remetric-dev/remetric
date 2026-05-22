# remetric demo

One-command demo of remetric against a synthetic Prometheus + Grafana stack
with a deliberately misbehaving metric (`cardinality-bomb`).

## Run

```bash
cd demo
docker compose up
```

Wait ~45 seconds. The `remetric-wait` container pauses 30s so Prometheus
accumulates a few scrape windows, then `remetric` runs once and writes an
HTML report.

When you see something like:

```
remetric-demo-remetric exited with code 0
```

open the report in your browser:

```bash
open output/report.html      # macOS
xdg-open output/report.html  # Linux
```

You should see findings for:

- `app_requests_total` flagged for high cardinality on `user_id` / `trace_id` / `path`
- `orphan_metric_total` flagged as unused (no dashboard or rule references it)
- Suspicious label patterns matching unbounded-identifier names
- An `always-firing` alert (provisioned by Prometheus alerting rules)

## What's in the stack

| Service | What it does |
|---------|--------------|
| `prometheus` (port 9090) | Scrapes the bomb + node-exporter every 5s |
| `node-exporter` (port 9100) | Host metrics, used to populate "unused metrics" findings |
| `cardinality-bomb` (port 8080) | Emits `app_requests_total{user_id,trace_id,path}` with 500 series and `orphan_metric_total` |
| `grafana` (port 3000, anonymous Admin) | Provisioned with one Prometheus datasource and a node dashboard |
| `remetric-wait` | One-shot Alpine container; `sleep 30` so the report sees real data |
| `remetric` | Runs `remetric report ... --format html --out /out/report.html` once, then exits |

The Prometheus and Grafana configs are reused from `../e2e/` so there is no
duplication; the demo composition only adds the `remetric-wait` + `remetric`
services on top.

## Tear down

```bash
docker compose down
rm -rf output/*.html
```

## Try other commands

The Grafana, Prometheus, and bomb services keep running while you experiment.
From the host (or any container on the `demo_default` network):

```bash
# Cardinality findings as JSON
remetric cardinality top \
  --prometheus http://localhost:9090 \
  --output json

# Suppress noise with --ignore
remetric scan \
  --prometheus http://localhost:9090 \
  --grafana http://localhost:3000 \
  --ignore-metric='node_.*'

# Markdown report (paste into a PR)
remetric report \
  --prometheus http://localhost:9090 \
  --grafana http://localhost:3000 \
  --format markdown > report.md
```

## Customising the demo

- **Use a local binary instead of the GHCR image.** Replace the `image:` line
  in the `remetric` service with `build: ..` and remove the volume mount path
  prefix if needed.
- **Shorter wait.** Drop `remetric-wait`'s sleep to `5` for impatient demos
  (fewer scrape cycles, fewer findings).
- **Different report format.** Change `--format=html` to `markdown` or `json`
  and update the `--out` extension accordingly.
