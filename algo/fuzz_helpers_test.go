/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package algo

import (
	"sort"

	"github.com/dgraph-io/dgraph/v25/protos/pb"
)

// bytesToUids interprets data as a sequence of uint64 values (little-endian).
// Any trailing bytes are dropped.
func bytesToUids(data []byte) []uint64 {
	n := len(data) / 8
	out := make([]uint64, n)
	for i := 0; i < n; i++ {
		v := uint64(0)
		for b := 0; b < 8; b++ {
			v |= uint64(data[i*8+b]) << (8 * b)
		}
		out[i] = v
	}
	return out
}

// bytesToUint64 reads a single little-endian uint64.
func bytesToUint64(data []byte) uint64 {
	if len(data) < 8 {
		v := uint64(0)
		for b := 0; b < len(data); b++ {
			v |= uint64(data[b]) << (8 * b)
		}
		return v
	}
	v := uint64(0)
	for b := 0; b < 8; b++ {
		v |= uint64(data[b]) << (8 * b)
	}
	return v
}

// intersectReference is the brute-force intersection used by fuzz tests as oracle.
func intersectReference(a, b []uint64) []uint64 {
	i, j := 0, 0
	out := []uint64{}
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			i++
		case a[i] > b[j]:
			j++
		default:
			out = append(out, a[i])
			i++
			j++
		}
	}
	return out
}

// differenceReference is brute-force u \ v.
func differenceReference(u, v []uint64) []uint64 {
	out := []uint64{}
	i, j := 0, 0
	for i < len(u) {
		if j >= len(v) {
			out = append(out, u[i])
			i++
			continue
		}
		switch {
		case u[i] < v[j]:
			out = append(out, u[i])
			i++
		case u[i] > v[j]:
			j++
		default:
			i++
			j++
		}
	}
	return out
}

// mergeReference is a naive k-way merge.
func mergeReference(lists []*pb.List) []uint64 {
	total := 0
	for _, l := range lists {
		total += len(l.Uids)
	}
	out := make([]uint64, 0, total)
	idx := make([]int, len(lists))
	for {
		best := -1
		var bestVal uint64
		for k, l := range lists {
			if idx[k] >= len(l.Uids) {
				continue
			}
			v := l.Uids[idx[k]]
			if best == -1 || v < bestVal {
				best = k
				bestVal = v
			}
		}
		if best == -1 {
			break
		}
		if len(out) == 0 || out[len(out)-1] != bestVal {
			out = append(out, bestVal)
		}
		idx[best]++
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// intersectSortedReference brute-force multi-list intersection.
func intersectSortedReference(lists []*pb.List) []uint64 {
	if len(lists) == 0 {
		return nil
	}
	if len(lists) == 1 {
		out := make([]uint64, len(lists[0].Uids))
		copy(out, lists[0].Uids)
		return out
	}
	a := lists[0].Uids
	b := intersectSortedReference(lists[1:])
	out := []uint64{}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			i++
		case a[i] > b[j]:
			j++
		default:
			out = append(out, a[i])
			i++
			j++
		}
	}
	return out
}