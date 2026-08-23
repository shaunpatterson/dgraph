/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package algo

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dgraph-io/dgraph/v25/protos/pb"
)

// dedupSorted returns a sorted, de-duplicated copy of uids.
func dedupSorted(uids []uint64) []uint64 {
	if len(uids) == 0 {
		return nil
	}
	out := make([]uint64, len(uids))
	copy(out, uids)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	w := 0
	for _, v := range out {
		if w == 0 || out[w-1] != v {
			out[w] = v
			w++
		}
	}
	return out[:w]
}

// FuzzIntersectWith asserts that all three intersection strategies agree and
// match the brute-force reference. Fuzz inputs are arbitrary byte sequences
// interpreted as pairs of uint64 slices.
func FuzzIntersectWith(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Split data into two uids slices using the first byte as a pivot.
		if len(data) < 2 {
			return
		}
		split := int(data[0]) % (len(data) - 1)
		if split < 0 {
			split = -split
		}
		split++
		raw1 := data[1:split]
		raw2 := data[split:]

		uids1 := bytesToUids(raw1)
		uids2 := bytesToUids(raw2)
		uids1 = dedupSorted(uids1)
		uids2 = dedupSorted(uids2)

		// Brute-force reference intersection.
		ref := intersectReference(uids1, uids2)

		// Linear intersection (small lists).
		outLin := &pb.List{Uids: []uint64{}}
		IntersectWithLin(uids1, uids2, &outLin.Uids)

		// Jump intersection (medium).
		outJump := &pb.List{Uids: []uint64{}}
		IntersectWithJump(uids1, uids2, &outJump.Uids)

		// Bin intersection (large).
		outBin := &pb.List{Uids: []uint64{}}
		IntersectWithBin(uids1, uids2, &outBin.Uids)

		require.Equal(t, ref, outLin.Uids, "IntersectWithLin mismatch")
		require.Equal(t, ref, outJump.Uids, "IntersectWithJump mismatch")
		require.Equal(t, ref, outBin.Uids, "IntersectWithBin mismatch")

		// Dispatching entry point.
		dispatched := &pb.List{Uids: []uint64{}}
		IntersectWith(&pb.List{Uids: uids1}, &pb.List{Uids: uids2}, dispatched)
		require.Equal(t, ref, dispatched.Uids, "IntersectWith dispatched mismatch")
	})
}

// FuzzMergeSorted asserts that MergeSorted and the brute-force reference agree.
func FuzzMergeSorted(f *testing.F) {
	f.Add([]byte{1, 5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{3, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 2 {
			return
		}
		nLists := int(data[0])%8 + 1
		data = data[1:]
		lists := []*pb.List{}
		for i := 0; i < nLists && len(data) >= 8; i++ {
			uids := bytesToUids(data)
			data = data[(len(uids))*8:]
			lists = append(lists, &pb.List{Uids: dedupSorted(uids)})
		}
		if len(lists) == 0 {
			return
		}

		// Brute-force merge reference.
		ref := mergeReference(lists)

		got := MergeSorted(lists)
		require.Equal(t, ref, got.Uids, "MergeSorted mismatch")

		got2 := MergeSortedMoreMem(lists)
		require.Equal(t, ref, got2.Uids, "MergeSortedMoreMem mismatch")
	})
}

// FuzzDifference asserts Difference matches brute-force reference.
func FuzzDifference(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{2, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 2 {
			return
		}
		split := int(data[0]) % (len(data) - 1)
		if split < 0 {
			split = -split
		}
		split++
		u1 := dedupSorted(bytesToUids(data[1:split]))
		u2 := dedupSorted(bytesToUids(data[split:]))

		ref := differenceReference(u1, u2)
		got := Difference(&pb.List{Uids: u1}, &pb.List{Uids: u2})
		require.Equal(t, ref, got.Uids, "Difference mismatch")
	})
}

// FuzzIndexOf asserts IndexOf matches the linear reference.
func FuzzIndexOf(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 8 {
			return
		}
		uids := dedupSorted(bytesToUids(data[:len(data)-8]))
		needle := bytesToUint64(data[len(data)-8:])

		// Linear reference.
		want := -1
		for i, v := range uids {
			if v == needle {
				want = i
				break
			}
		}
		got := IndexOf(&pb.List{Uids: uids}, needle)
		require.Equal(t, want, got, "IndexOf mismatch")
	})
}

// FuzzIntersectSorted asserts IntersectSorted matches brute-force reference.
func FuzzIntersectSorted(f *testing.F) {
	f.Add([]byte{2, 1, 2, 3, 4, 5, 6, 7})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 2 {
			return
		}
		nLists := int(data[0])%5 + 1
		data = data[1:]
		var lists []*pb.List
		for i := 0; i < nLists && len(data) >= 8; i++ {
			uids := bytesToUids(data)
			data = data[(len(uids))*8:]
			lists = append(lists, &pb.List{Uids: dedupSorted(uids)})
		}
		if len(lists) == 0 {
			return
		}
		ref := intersectSortedReference(lists)
		got := IntersectSorted(lists)
		require.Equal(t, ref, got.Uids, "IntersectSorted mismatch")
	})
}