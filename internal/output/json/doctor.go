// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package json

import "github.com/remetric-dev/remetric/internal/findings"

// RenderDoctor emits the findings.DoctorReport as indented JSON.
func (r *Renderer) RenderDoctor(rep findings.DoctorReport) error {
	return r.encode(rep)
}
