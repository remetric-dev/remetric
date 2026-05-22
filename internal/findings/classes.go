// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

// Class is the stable kebab-case slug identifying a finding's type
// (independent of the per-instance ID, which encodes the offending entity).
//
// Each Class maps 1:1 to a page at remetric.dev/findings/<class>. Renaming
// a Class is a public-API break - update the matching docs/findings/<class>.md
// in the same change.
type Class = string

// Known finding classes. The kebab-case slug is the wire form and the URL slug.
const (
	ClassHotLabel                   Class = "hot-label"
	ClassUnusedMetric               Class = "unused-metric"
	ClassLabelPatternOverlyGranular Class = "label-pattern-overly-granular"
	ClassNeverFiringAlert           Class = "never-firing-alert"
	ClassAlwaysFiringAlert          Class = "always-firing-alert"
	ClassBrokenPanel                Class = "broken-panel"
)

const docsBaseURL = "https://remetric.dev/findings/"

// DocURL returns the public reference URL for class. Empty class returns "".
func DocURL(class Class) string {
	if class == "" {
		return ""
	}
	return docsBaseURL + class
}

// FillDocURLs populates DocURL on each finding whose Class is set and whose
// DocURL is still empty. Findings with an empty Class or a caller-provided
// DocURL are left untouched. It edits the slice in place.
//
// FillDocURLs centralises the URL-derivation rule so analyzers do not have
// to hard-code the docs domain.
func FillDocURLs(fs []Finding) {
	for i := range fs {
		if fs[i].Class != "" && fs[i].DocURL == "" {
			fs[i].DocURL = DocURL(fs[i].Class)
		}
	}
}
