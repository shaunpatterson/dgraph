/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package schema

import (
	"math"
	"testing"
)

// FuzzSchemaParse exercises schema.Parse against arbitrary input.
func FuzzSchemaParse(f *testing.F) {
	f.Add("")
	f.Add("\n")
	f.Add("name: string .")
	f.Add("name: int .")
	f.Add("name: string @index(term) .")
	f.Add("name: [string] .")
	f.Add("type Person { name }")
	f.Add("[0x1] name: string .")
	f.Add("type X { field: string }")
	f.Add("name: int @index(int) .")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("schema.Parse panicked on %q: %v", s, r)
			}
		}()
		// Reset state before each call so schema state doesn't leak between fuzz inputs.
		reset()
		_, _ = Parse(s)
	})
}

// FuzzParseWithNamespace exercises ParseWithNamespace with various namespaces.
func FuzzParseWithNamespace(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(math.MaxUint64))

	f.Fuzz(func(t *testing.T, ns uint64) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseWithNamespace panicked on ns=%d: %v", ns, r)
			}
		}()
		reset()
		_, _ = ParseWithNamespace("name: string .", ns)
	})
}

// FuzzParseBytes exercises ParseBytes against arbitrary input.
func FuzzParseBytes(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("name: string ."))
	f.Add([]byte("name: int @index(term) ."))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseBytes panicked on %q: %v", data, r)
			}
		}()
		reset()
		_ = ParseBytes(data, 0)
	})
}