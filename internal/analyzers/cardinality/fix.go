// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

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

type fixData struct {
	Metric string
	Label  string
}

func renderFix(metric, label string) (string, error) {
	var buf bytes.Buffer
	if err := fixTmpl.Execute(&buf, fixData{Metric: metric, Label: label}); err != nil {
		return "", fmt.Errorf("render fix: %w", err)
	}
	return buf.String(), nil
}
