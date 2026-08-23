/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package lex

import "testing"

// FuzzLexer is a generic fuzz for the lexer. It should never panic.
func FuzzLexer(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("abc"))
	f.Add([]byte("<a>"))
	f.Add([]byte("\"hello\\nworld\""))
	f.Add([]byte("\"\\uD800\""))
	f.Add([]byte("\"\\U00000000\""))
	f.Add([]byte("\"\\xFF\""))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Lexer panicked on %q: %v", data, r)
			}
		}()
		l := &Lexer{}
		l.Reset(string(data))
		// Run any reasonable state function (or none); we just exercise the
		// lexer's internal primitives.
		l.Run(defaultState)
	})
}

// defaultState is a trivial StateFn that just consumes until EOF so the
// fuzz can poke at Backup/Next/Peek/Emit on the freshly Reset lexer.
func defaultState(l *Lexer) StateFn {
	for {
		r := l.Next()
		if r == EOF {
			l.Emit(ItemEOF)
			return nil
		}
	}
}

// FuzzLexQuotedString ensures LexQuotedString never panics.
func FuzzLexQuotedString(f *testing.F) {
	f.Add([]byte("\"\""))
	f.Add([]byte("\"a\""))
	f.Add([]byte("\"\\n\""))
	f.Add([]byte("\"\\"))
	f.Add([]byte("\""))
	f.Add([]byte("\"\\u123\""))
	f.Add([]byte("\"abc"))
	f.Add([]byte("\"\\q\""))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LexQuotedString panicked on %q: %v", data, r)
			}
		}()
		l := &Lexer{}
		l.Reset(string(data))
		// Read the first rune so LexQuotedString's initial Backup makes sense.
		if r := l.Next(); r == EOF {
			return
		}
		_ = l.LexQuotedString()
	})
}

// FuzzIRIRef ensures IRIRef never panics on weird inputs.
func FuzzIRIRef(f *testing.F) {
	f.Add([]byte("<>"))
	f.Add([]byte("<a>"))
	f.Add([]byte("<\u0000>"))
	f.Add([]byte("<\\\\u1234>"))
	f.Add([]byte("<\\\\U00000000>"))
	f.Add([]byte("<\\\\x00>"))
	f.Add([]byte("<\\\\>"))
	f.Add([]byte("<"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("IRIRef panicked on %q: %v", data, r)
			}
		}()
		l := &Lexer{}
		l.Reset(string(data))
		_ = IRIRef(l, ItemError)
	})
}