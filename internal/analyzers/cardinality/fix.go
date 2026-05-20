package cardinality

import (
	"bytes"
	"fmt"
	"text/template"
)

const fixTemplate = `metric_relabel_configs:
  - source_labels: [__name__]
    regex: "{{.Metric}}"
    action: keep
  - regex: "{{.Label}}"
    action: labeldrop
`

var fixTmpl = template.Must(template.New("fix").Parse(fixTemplate))

func renderFix(metric, label string) (string, error) {
	var buf bytes.Buffer
	if err := fixTmpl.Execute(&buf, struct{ Metric, Label string }{metric, label}); err != nil {
		return "", fmt.Errorf("render fix: %w", err)
	}
	return buf.String(), nil
}
