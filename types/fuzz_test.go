/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package types

import (
	"math"
	"testing"
)

// FuzzParseVFloat exercises the vfloat parser against arbitrary strings.
func FuzzParseVFloat(f *testing.F) {
	f.Add("[]")
	f.Add("[1.0]")
	f.Add("[1.0, 2.0]")
	f.Add("[1.0 2.0 3.0]")
	f.Add(" [ 1.0 , 2.0 ] ")
	f.Add("[NaN]")
	f.Add("[Inf]")
	f.Add("[-Inf]")
	f.Add("[1e999]")
	f.Add("[1e-999]")
	f.Add("[\"x\"]")
	f.Add("[1.0,")
	f.Add("[1,2,]")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseVFloat panicked on %q: %v", s, r)
			}
		}()
		out, err := ParseVFloat(s)
		if err != nil {
			return
		}
		for i, v := range out {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("ParseVFloat returned non-finite value at %d for input %q: %v", i, s, v)
			}
		}
	})
}

// FuzzTypeForValue checks that TypeForValue never panics.
func FuzzTypeForValue(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("123"))
	f.Add([]byte("1.5"))
	f.Add([]byte("true"))
	f.Add([]byte("false"))
	f.Add([]byte("2020-01-01"))
	f.Add([]byte("2020-01-01T00:00:00Z"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("TypeForValue panicked on %q: %v", data, r)
			}
		}()
		TypeForValue(data)
	})
}

// FuzzConvertString checks Convert never panics on string→X for arbitrary input.
func FuzzConvertString(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("123"))
	f.Add([]byte("-9223372036854775808"))
	f.Add([]byte("9223372036854775807"))
	f.Add([]byte("1.7976931348623157e+308"))
	f.Add([]byte("-1.7976931348623157e+308"))
	f.Add([]byte("NaN"))
	f.Add([]byte("Inf"))
	f.Add([]byte("-Inf"))
	f.Add([]byte("true"))
	f.Add([]byte("0"))
	f.Add([]byte("not-a-date"))
	f.Add([]byte("POINT(1 2)"))
	f.Add([]byte("{\"type\":\"Point\",\"coordinates\":[1,2]}"))
	f.Add([]byte("[1.0,2.0,3.0]"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Convert panicked on %q: %v", data, r)
			}
		}()
		from := Val{Tid: StringID, Value: data}
		for _, to := range []TypeID{BinaryID, IntID, FloatID, BoolID, DateTimeID, VFloatID, BigFloatID, PasswordID} {
			_, _ = Convert(from, to)
		}
	})
}

// FuzzConvertBinary checks Convert from BinaryID to other types never panics.
func FuzzConvertBinary(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{1})
	f.Add([]byte{2})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0x7f, 0xef, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Convert(BinaryID) panicked on %x: %v", data, r)
			}
		}()
		from := Val{Tid: BinaryID, Value: data}
		for _, to := range []TypeID{IntID, FloatID, BoolID, DateTimeID, VFloatID, BigFloatID, PasswordID, StringID} {
			_, _ = Convert(from, to)
		}
	})
}


// FuzzLessEqual tests that Less and Equal are consistent.
func FuzzLessEqual(f *testing.F) {
	f.Add(int64(1), int64(1))
	f.Add(int64(1), int64(2))
	f.Add(int64(-1), int64(1))

	f.Fuzz(func(t *testing.T, a, b int64) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Less/Equal panicked: %v", r)
			}
		}()
		va := Val{Tid: IntID, Value: a}
		vb := Val{Tid: IntID, Value: b}
		less, _ := Less(va, vb)
		lessR, _ := Less(vb, va)
		eq, _ := Equal(va, vb)
		if less && lessR {
			t.Fatalf("Less is not antisymmetric for %d vs %d", a, b)
		}
		if eq != (!less && !lessR) {
			t.Fatalf("Less/Equal inconsistent for %d vs %d: less=%v lessR=%v eq=%v",
				a, b, less, lessR, eq)
		}
	})
}

// FuzzParseTime checks ParseTime doesn't panic on arbitrary strings.
func FuzzParseTime(f *testing.F) {
	f.Add("2020-01-01")
	f.Add("2020-01-01T00:00:00Z")
	f.Add("")
	f.Add("not-a-time")

	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseTime panicked on %q: %v", s, r)
			}
		}()
		_, _ = ParseTime(s)
	})
}
