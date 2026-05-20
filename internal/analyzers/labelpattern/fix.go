// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"bytes"
	"text/template"
)

const fixTemplate = `metric_relabel_configs:
  - regex: '{{.Label}}'
    action: labeldrop
# Affected metrics ({{ len .Metrics }}):
{{- range .Metrics }}
#   - {{ . }}
{{- end }}
`

var fixTmpl = template.Must(template.New("labeldrop").Parse(fixTemplate))

// RenderFix produces a Prometheus metric_relabel_configs snippet that
// drops label across the listed metrics. The snippet is per-job; the
// operator decides which jobs to apply it to.
func RenderFix(label string, metrics []string) (string, error) {
	var buf bytes.Buffer
	data := struct {
		Label   string
		Metrics []string
	}{Label: label, Metrics: metrics}
	if err := fixTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
