/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package dql

import "testing"

// FuzzParse exercises the DQL parser with arbitrary input beyond the corpus
// tarball. The existing parser_fuzz_test.go uses an explicit seed corpus; this
// variant relies purely on fuzz-generated inputs to surface new coverage.
func FuzzParse(f *testing.F) {
	f.Add("")
	f.Add("{}")
	f.Add("{ }")
	f.Add("{} block {}")
	f.Add("{} @filter")
	f.Add("{ a(func: eq(name, \"x\")) {} }")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Parse panicked on %q: %v", s, r)
			}
		}()
		_, _ = Parse(Request{Str: s})
	})
}