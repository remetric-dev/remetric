// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package json

import (
	stdjson "encoding/json"
	"fmt"
	"io"

	"github.com/remetric-dev/remetric/internal/findings"
)

// Renderer encodes scan output as indented JSON.
type Renderer struct {
	w io.Writer
}

// New constructs a Renderer that writes indented JSON to w.
func New(w io.Writer) *Renderer { return &Renderer{w: w} }

// RenderFindings emits the minimal { "findings": [...], "summary": {...} }
// envelope for focused subcommands. Summary is computed on the fly from fs.
func (r *Renderer) RenderFindings(fs []findings.Finding) error {
	if fs == nil {
		fs = []findings.Finding{}
	}
	rep := findings.NewReport("", fs)
	envelope := struct {
		Findings []findings.Finding `json:"findings"`
		Summary  findings.Summary   `json:"summary"`
	}{
		Findings: fs,
		Summary:  rep.Summary,
	}
	return r.encode(envelope)
}

// RenderReport emits the full §5.5 schema.
func (r *Renderer) RenderReport(rep *findings.Report) error {
	if rep == nil {
		return fmt.Errorf("output/json: nil report")
	}
	if rep.Findings == nil {
		rep.Findings = []findings.Finding{}
	}
	return r.encode(rep)
}

// RenderLabelInventory emits the cardinality-labels inventory.
func (r *Renderer) RenderLabelInventory(inv LabelInventory) error {
	if inv.Labels == nil {
		inv.Labels = []LabelStat{}
	}
	return r.encode(inv)
}

func (r *Renderer) encode(v any) error {
	enc := stdjson.NewEncoder(r.w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
