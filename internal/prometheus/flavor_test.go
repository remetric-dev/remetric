// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import "testing"

func TestFlavor_String(t *testing.T) {
	cases := []struct {
		in   Flavor
		want string
	}{
		{FlavorUnknown, "unknown"},
		{FlavorProm, "prometheus"},
		{FlavorVictoria, "victoria"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("Flavor(%d).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWithFlavorHint_SetsField(t *testing.T) {
	c, err := New("http://localhost:9090", WithFlavorHint(FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.flavorHint != FlavorVictoria {
		t.Errorf("flavorHint = %v, want FlavorVictoria", c.flavorHint)
	}
}
