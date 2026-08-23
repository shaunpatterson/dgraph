/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package chunker

import (
	"sort"
	"testing"

	"github.com/dgraph-io/dgo/v250/protos/api"
	"github.com/dgraph-io/simdjson-go"
	"github.com/stretchr/testify/require"
)

// FuzzJSONParserDifferential compares ParseJSON (encoding/json) and
// FastParseJSON (simdjson-go) on the same input. Both must agree on:
//  1. accept/reject parity (no parser silently swallows a malformed input);
//  2. the resulting set of NQuads (sorted by (Subject, Predicate) to be
//     order-insensitive).
//
// FastParseJSON falls back to ParseJSON when simdjson.SupportedCPU() is
// false, so we skip on unsupported CPUs — otherwise we'd be comparing a
// function against itself.
func FuzzJSONParserDifferential(f *testing.F) {
	f.Add(`[{"uid":"0x1","name":"Alice"}]`)
	f.Add(`[{"uid":"0x1","name":"Alice","friend":[{"uid":"0x2","name":"Bob"}]}]`)
	f.Add(`[{"uid":"0x1","name|hi":"Namaste","name":"Alice"}]`)
	f.Add(`[]`)

	f.Fuzz(func(t *testing.T, s string) {
		if !simdjson.SupportedCPU() {
			t.Skip("simdjson unsupported; FastParseJSON degenerates to ParseJSON")
		}
		buf1 := NewNQuadBuffer(1000)
		err1 := buf1.ParseJSON([]byte(s), SetNquads)
		buf2 := NewNQuadBuffer(1000)
		err2 := buf2.FastParseJSON([]byte(s), SetNquads)

		// (1) Accept/reject parity.
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("accept/reject mismatch on %q: ParseJSON err=%v, FastParseJSON err=%v",
				s, err1, err2)
		}
		if err1 != nil {
			return
		}

		// (2) Sorted NQuad contents must match.
		got1 := buf1.nquads
		got2 := buf2.nquads
		require.Equal(t, len(got1), len(got2),
			"length mismatch: ParseJSON=%d, FastParseJSON=%d for %q",
			len(got1), len(got2), s)
		sortNQuads(got1)
		sortNQuads(got2)
		for i := range got1 {
			require.True(t, nquadEqual(got1[i], got2[i]),
				"NQuad[%d] mismatch for %q:\n  ParseJSON: %+v\n  FastParseJSON: %+v",
				i, s, got1[i], got2[i])
		}
	})
}

func sortNQuads(ns []*api.NQuad) {
	sort.Slice(ns, func(i, j int) bool {
		a, b := ns[i], ns[j]
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		if a.Predicate != b.Predicate {
			return a.Predicate < b.Predicate
		}
		return a.ObjectId < b.ObjectId
	})
}

func nquadEqual(a, b *api.NQuad) bool {
	if a.Subject != b.Subject || a.Predicate != b.Predicate ||
		a.ObjectId != b.ObjectId || a.Lang != b.Lang {
		return false
	}
	if a.Namespace != b.Namespace {
		return false
	}
	if (a.ObjectValue == nil) != (b.ObjectValue == nil) {
		return false
	}
	if a.ObjectValue != nil && b.ObjectValue != nil {
		// Compare the embedded DefaultVal as the simplest stable field; the
		// parsers should agree on which branch of the oneof they pick.
		da := a.ObjectValue.GetDefaultVal()
		db := b.ObjectValue.GetDefaultVal()
		return da == db
	}
	return true
}