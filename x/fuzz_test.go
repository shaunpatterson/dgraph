/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package x

import (
	"bufio"
	"bytes"
	"testing"
)

// FuzzParseAttr exercises ParseAttr against arbitrary strings.
func FuzzParseAttr(f *testing.F) {
	f.Add("0-name")
	f.Add("name")
	f.Add("")
	f.Add("-")
	f.Add("0-")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseAttr panicked on %q: %v", s, r)
			}
		}()
		_ = ParseAttr(s)
	})
}

// FuzzParseNamespace exercises ParseNamespace against arbitrary strings.
func FuzzParseNamespace(f *testing.F) {
	f.Add("0-name")
	f.Add("name")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseNamespace panicked on %q: %v", s, r)
			}
		}()
		_ = ParseNamespace(s)
	})
}

// FuzzParseNamespaceAttr exercises ParseNamespaceAttr against arbitrary strings.
func FuzzParseNamespaceAttr(f *testing.F) {
	f.Add("0-name")
	f.Add("name")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseNamespaceAttr panicked on %q: %v", s, r)
			}
		}()
		_, _ = ParseNamespaceAttr(s)
	})
}

// FuzzNamespaceAttrRoundTrip asserts NamespaceAttr is well-formed and parses
// back via ParseAttr without panicking.
func FuzzNamespaceAttrRoundTrip(f *testing.F) {
	f.Add(uint64(0), "name")
	f.Add(uint64(255), "name")

	f.Fuzz(func(t *testing.T, ns uint64, attr string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("round-trip panicked on ns=%d attr=%q: %v", ns, attr, r)
			}
		}()
		combined := NamespaceAttr(ns, attr)
		if combined == "" {
			t.Fatalf("NamespaceAttr returned empty for ns=%d attr=%q", ns, attr)
		}
		if combined != "" {
			gotAttr := ParseAttr(combined)
			if gotAttr != attr {
				t.Fatalf("ParseAttr mismatch: got %q, want %q (combined=%q)", gotAttr, attr, combined)
			}
		}
	})
}

// FuzzValidateAddress exercises the address validator.
func FuzzValidateAddress(f *testing.F) {
	f.Add("127.0.0.1:8080")
	f.Add("[::1]:8080")
	f.Add("localhost:80")
	f.Add("host:0")
	f.Add("host:-1")
	f.Add("host:99999")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ValidateAddress panicked on %q: %v", s, r)
			}
		}()
		_ = ValidateAddress(s)
	})
}

// FuzzReadLine exercises the bufio-backed ReadLine helper.
func FuzzReadLine(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("\n"))
	f.Add([]byte("hello\n"))
	f.Add([]byte("hello world\nfoo\n"))
	f.Add([]byte("\r\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ReadLine panicked: %v", r)
			}
		}()
		r := bufio.NewReader(bytes.NewReader(data))
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			err := ReadLine(r, &buf)
			if err != nil {
				break
			}
		}
	})
}