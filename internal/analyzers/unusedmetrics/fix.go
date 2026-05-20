// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package unusedmetrics flags Prometheus metrics that no Grafana
// dashboard, alert rule, or recording rule references.
package unusedmetrics

import (
	"bytes"
	"text/template"
)

const fixTemplate = `metric_relabel_configs:
  - source_labels: [__name__]
    regex: '{{.Metric}}'
    action: drop
`

var fixTmpl = template.Must(template.New("dropmetric").Parse(fixTemplate))

// RenderFix produces a metric_relabel_configs snippet that drops the
// named metric. The snippet is per scrape-job; the operator applies
// it where it makes sense.
func RenderFix(metric string) (string, error) {
	var buf bytes.Buffer
	if err := fixTmpl.Execute(&buf, struct{ Metric string }{metric}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
