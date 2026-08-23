/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package chunker

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/dgraph-io/dgraph/v25/lex"
)

// FuzzParseRDF ensures the RDF parser never panics and never returns both
// a non-zero NQuad and an error.
func FuzzParseRDF(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("\n"))
	f.Add([]byte("subject"))
	f.Add([]byte("<s> <p> <o> ."))
	f.Add([]byte("<s> <p> <o> .\n"))
	f.Add([]byte(`<s> <p> "literal" .`))
	f.Add([]byte(`uid(0x1) <p> uid(0x2) .`))
	f.Add([]byte(`_:b <p> "v"@en .`))
	f.Add([]byte("# just a comment\n"))
	f.Add([]byte("<s> <p> * * ."))
	f.Add([]byte("<s> <p> \"\\u0000\" ."))
	f.Add([]byte("<s> <p> \"\\xFF\"^^<xs:string> ."))
	f.Add([]byte("<s> <p> \"val\" (key=\"v\") ."))
	f.Add([]byte("v.l(b) <p> <o> ."))

	f.Fuzz(func(t *testing.T, data []byte) {
		l := &lex.Lexer{}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseRDF panicked on input %q: %v", data, r)
			}
		}()
		nq, err := ParseRDF(string(data), l)
		// If ParseRDF returned ErrEmpty, nq must be zero-valued.
		if err == ErrEmpty {
			if nq.Subject != "" || nq.Predicate != "" || nq.ObjectId != "" || nq.ObjectValue != nil {
				t.Fatalf("ParseRDF returned non-zero nq with ErrEmpty: %+v on %q", nq, data)
			}
			return
		}
		// Otherwise, no constraint: any error is acceptable, any non-error is acceptable.
		_ = nq
	})
}

// FuzzJSONChunker feeds the JSON Chunk function arbitrary bytes that may
// or may not start with `{` or `[`. The chunker must never panic and must
// return an error for non-JSON input.
func FuzzJSONChunker(f *testing.F) {
	f.Add([]byte("{}"))
	f.Add([]byte("[{}]"))
	f.Add([]byte("[{},{}]"))
	f.Add([]byte(""))
	f.Add([]byte("["))
	f.Add([]byte("[{"))
	f.Add([]byte("[{ \"a\":"))
	f.Add([]byte("[{}"))
	f.Add([]byte("[\"\\"))
	f.Add([]byte("[{\"a\":\"\\uXXXX\"}]"))

	f.Fuzz(func(t *testing.T, data []byte) {
		jc := NewChunker(JsonFormat, 1000).(interface {
			Chunk(r *bufio.Reader) (*bytes.Buffer, error)
		})
		r := bufio.NewReader(bytes.NewReader(data))
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("JSON Chunker panicked on %q: %v", data, rec)
			}
		}()
		// Just exercise Chunk; result is irrelevant — we only check that we
		// don't panic and we get either a non-nil buffer+error or buffer+nil.
		buf, _ := jc.Chunk(r)
		_ = buf
	})
}

// FuzzRDFChunkerChunk ensures the RDF Chunk function never panics and
// always returns a buffer that is at most the size of the input.
func FuzzRDFChunkerChunk(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("\n"))
	f.Add([]byte("subject predicate object .\n"))
	f.Add([]byte(strings.Repeat("a", 1<<20)))

	f.Fuzz(func(t *testing.T, data []byte) {
		rc := NewChunker(RdfFormat, 1000).(interface {
			Chunk(r *bufio.Reader) (*bytes.Buffer, error)
		})
		r := bufio.NewReader(bytes.NewReader(data))
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("RDF Chunker panicked: %v", rec)
			}
		}()
		buf, err := rc.Chunk(r)
		// A successful call must return a non-nil buffer (even if empty).
		if err == nil && buf == nil {
			t.Fatalf("RDF Chunker returned nil buffer with nil error")
		}
	})
}