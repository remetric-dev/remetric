// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package dashboardhygiene

import (
	"fmt"
	"strings"
)

// buildFix renders a paste-ready instruction block for a single
// (dashboard, missing-metric) finding. The block opens with a header
// identifying the dashboard (with optional absolute URL), lists the
// two remediation paths (restore the metric or remove the queries),
// names the affected panels (up to maxFixPanels, then "... and N more"),
// and closes with a Reference link to the public docs page.
func buildFix(dashTitle, missing, dashURL string, panelTitles []string) string {
	var b strings.Builder
	if dashURL != "" {
		fmt.Fprintf(&b, "Edit dashboard %q (%s)\n", dashTitle, dashURL)
	} else {
		fmt.Fprintf(&b, "Edit dashboard %q\n", dashTitle)
	}
	b.WriteString("and either:\n")
	fmt.Fprintf(&b, "  1. Restore metric %q (re-enable the scrape job or recording rule), or\n", missing)
	b.WriteString("  2. Remove/replace the broken queries in panel(s):\n")
	n := len(panelTitles)
	limit := n
	if limit > maxFixPanels {
		limit = maxFixPanels
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintf(&b, "     - %s\n", panelTitles[i])
	}
	if n > maxFixPanels {
		fmt.Fprintf(&b, "     ... and %d more\n", n-maxFixPanels)
	}
	b.WriteString("Reference: https://remetric.dev/findings/broken-panel")
	return b.String()
}

// maxFixPanels caps the number of panel titles listed in the Fix
// snippet. Beyond this, panel titles are elided with "... and N more"
// to keep the output skimmable.
const maxFixPanels = 10
