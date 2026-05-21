// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package html

import (
	_ "embed"
	"html/template"
)

//go:embed template.html
var templateSrc string

//go:embed style.css
var styleSrc string

//go:embed script.js
var scriptSrc string

var rendererTemplate = template.Must(template.New("report").Parse(templateSrc))
