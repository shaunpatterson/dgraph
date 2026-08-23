/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package tok

import (
	"math/big"
	"testing"
)

// FuzzTokenizerTypeAssertion exercises BuildTokens with arbitrary interface{}
// values to surface type-assertion panics in concrete tokenizers.
func FuzzTokenizerTypeAssertion(f *testing.F) {
	f.Add(int64(1))
	f.Add(int64(2))
	f.Add(int64(-1))
	f.Add(int64(0))
	f.Fuzz(func(t *testing.T, val int64) {
		// Try each tokenizer with several different concrete types.
		cases := []struct {
			tok  Tokenizer
			val interface{}
		}{
			{IntTokenizer{}, val},
			{IntTokenizer{}, "wrong type"},
			{IntTokenizer{}, float64(1)},
			{FloatTokenizer{}, float64(1)},
			{FloatTokenizer{}, int64(1)},
			{FloatTokenizer{}, "wrong"},
			{BigFloatTokenizer{}, big.NewFloat(1)},
			{BigFloatTokenizer{}, "wrong"},
		}
		for _, c := range cases {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("BuildTokens panicked on %T: %v", c.val, r)
					}
				}()
				_, _ = BuildTokens(c.val, c.tok)
			}()
		}
	})
}

// FuzzBuildTokensRoundTrip exercises BuildTokens with arbitrary string input.
func FuzzBuildTokensRoundTrip(f *testing.F) {
	f.Add("")
	f.Add("hello")
	f.Add("hello world")
	f.Add("\x00\x00")

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 256 {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildTokens panicked on %q: %v", s, r)
			}
		}()
		tok, has := GetTokenizer("term")
		if !has {
			t.Skip("term tokenizer not registered")
		}
		_, _ = BuildTokens(s, tok)
	})
}