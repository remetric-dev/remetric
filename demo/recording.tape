# demo/recording.tape - vhs script that produces demo/remetric.gif
#
# Prerequisites:
#   1. `make build` (puts remetric on $PWD/bin/remetric)
#   2. `cd demo && docker compose up -d && sleep 45`
#
# Generate from repo root with:
#   PATH="$PWD/bin:$PATH" vhs demo/recording.tape
#
# Edit this file, re-run vhs - the gif is regenerated deterministically.

Output demo/remetric.gif

Set Theme "Dracula"
Set FontSize 17
Set Width 1280
Set Height 760
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false

# Frame 1: quick scan against the demo stack (medium+ severity = focused output)
Type "remetric scan --prometheus http://localhost:9091 --grafana http://localhost:3001 --min-severity medium"
Sleep 600ms
Enter
Sleep 9s

# Frame 2: --ignore filter removes known noisy metric families
Hide
Type "clear"
Enter
Show
Sleep 300ms
Type "# Suppress known noise with --ignore-metric (regex, repeatable):"
Enter
Sleep 700ms
Type "remetric scan --prometheus http://localhost:9091 --grafana http://localhost:3001 --ignore-metric='node_.*' --ignore-metric='prometheus_.*' --min-severity low | tail -8"
Sleep 600ms
Enter
Sleep 9s

# Frame 3: machine-readable JSON for CI / dashboards
Hide
Type "clear"
Enter
Show
Sleep 300ms
Type "# Machine-readable JSON for CI pipelines:"
Enter
Sleep 700ms
Type "remetric cardinality top --prometheus http://localhost:9091 --limit 1 --output json"
Sleep 600ms
Enter
Sleep 6s
