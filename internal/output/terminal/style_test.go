// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package terminal

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestStyleForSeverity_AllCovered(t *testing.T) {
	for _, s := range []findings.Severity{findings.SeverityLow, findings.SeverityMedium, findings.SeverityHigh, findings.SeverityCritical} {
		st := newStyles(false).severity(s)
		_ = st.Render(s.String())
	}
}

func TestStyles_NoColorProducesNoAnsi(t *testing.T) {
	st := newStyles(true)
	out := st.severity(findings.SeverityCritical).Render("CRITICAL")
	if out != "CRITICAL" {
		t.Errorf("noColor render = %q, want plain %q", out, "CRITICAL")
	}
}
