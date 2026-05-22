# remetric demo

One-command demo of remetric against a synthetic Prometheus + Grafana stack
with a deliberately misbehaving metric (`cardinality-bomb`).

## Run

```bash
cd demo
docker compose up -d         # start everything in the background
docker compose wait remetric # blocks ~45s until the report is written
open output/report.html      # macOS  (use xdg-open on Linux)
docker compose down          # tear down when done
```

What happens:

1. Prometheus, Grafana, node-exporter, and cardinality-bomb start.
2. `remetric-wait` sleeps 30 seconds so Prometheus accumulates a few scrape
   windows (otherwise the report would be empty).
3. `remetric` runs once: `report --prometheus=http://prometheus:9090
   --grafana=http://grafana:3000 --format=html --out=/out/report.html`.
4. The report appears in `./output/report.html`. Other services stay up so
   you can poke at them.

You should see findings for:

- `app_requests_total` flagged for high cardinality on `user_id` / `trace_id` / `path`
- `orphan_metric_total` flagged as unused (no dashboard or rule references it)
- Suspicious label patterns matching unbounded-identifier names
- An `always-firing` alert (provisioned by Prometheus alerting rules)

## What's in the stack

| Service | Host port | What it does |
|---------|-----------|--------------|
| `prometheus` | 9091 | Scrapes the bomb + node-exporter every 5s |
| `node-exporter` | 9101 | Host metrics, used to populate "unused metrics" findings |
| `cardinality-bomb` | 8081 | Emits `app_requests_total{user_id,trace_id,path}` (500 series) and `orphan_metric_total` |
| `grafana` (anonymous Admin) | 3001 | Provisioned with one Prometheus datasource and a node dashboard |
| `remetric-wait` | - | One-shot Alpine container; `sleep 30` so the report sees real data |
| `remetric` | - | Runs `remetric report ... --format html --out /out/report.html` once, then exits |

Host ports use the `*1` suffix (`9091/9101/8081/3001`) so the demo can run
in parallel with `make e2e-up` (which uses `9090/9100/8080/3000`). Container
network references unchanged - `remetric` talks to `prometheus:9090` and
`grafana:3000` internally.

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
  --prometheus http://localhost:9091 \
  --output json

# Suppress noise with --ignore
remetric scan \
  --prometheus http://localhost:9091 \
  --grafana http://localhost:3001 \
  --ignore-metric='node_.*'

# Markdown report (paste into a PR)
remetric report \
  --prometheus http://localhost:9091 \
  --grafana http://localhost:3001 \
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

## Regenerating the README GIF

The README animation [`remetric.gif`](remetric.gif) is rendered from
[`recording.tape`](recording.tape) via [vhs](https://github.com/charmbracelet/vhs).

```bash
brew install vhs ffmpeg                              # one-time
make build                                           # produces ./bin/remetric
cd demo && docker compose up -d && sleep 45 && cd .. # stack must be live + scraped
PATH="$PWD/bin:$PATH" vhs demo/recording.tape        # writes demo/remetric.gif
```

Both the `.tape` script and the rendered `.gif` are committed so the recording
is deterministic - edit the tape, re-run, the gif regenerates byte-for-byte.
