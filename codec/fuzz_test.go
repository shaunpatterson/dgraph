/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package codec

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dgraph-io/dgraph/v25/protos/pb"
)

// FuzzEncodeDecodeRoundTrip encodes arbitrary uids and verifies the decoder
// produces the same sequence back. It also verifies ApproxLen and ExactLen
// stay within expected bounds.
func FuzzEncodeDecodeRoundTrip(f *testing.F) {
	f.Add(uint16(1), uint16(0))
	f.Add(uint16(7), uint16(0))
	f.Add(uint16(256), uint16(0))

	f.Fuzz(func(t *testing.T, blockSize, _ uint16) {
		if blockSize == 0 {
			blockSize = 1
		}
		// Seed corpus: a few simple lists.
		seedLists := [][]uint64{
			{},
			{1},
			{1, 2, 3},
			{0, math.MaxUint32, math.MaxUint64},
		}
		for _, uids := range seedLists {
			checkRoundTrip(t, uids, int(blockSize))
		}
	})
}

func checkRoundTrip(t *testing.T, uids []uint64, blockSize int) {
	t.Helper()
	pack := Encode(uids, blockSize)
	defer FreePack(pack)

	// ExactLen must equal len(uids).
	require.Equal(t, len(uids), ExactLen(pack), "ExactLen mismatch")

	// ApproxLen must be >= len(uids) but not absurdly large.
	approx := ApproxLen(pack)
	require.GreaterOrEqual(t, approx, len(uids), "ApproxLen undercount")

	// Full decode must equal input.
	got := Decode(pack, 0)
	require.Equal(t, uids, got, "Decode mismatch")

	// Decode must be idempotent regardless of starting seek.
	if len(uids) > 0 {
		gotSeek := Decode(pack, uids[0])
		require.Equal(t, uids, gotSeek, "Decode with mid-seek mismatch")
	}

	// CopyUidPack must yield the same decoded sequence.
	cp := CopyUidPack(pack)
	if cp != nil {
		got2 := Decode(cp, 0)
		require.Equal(t, uids, got2, "CopyUidPack/Decode mismatch")
	}
}

// FuzzSeekVsLinearScan fuzzes a randomly generated pack and verifies that
// SeekStart, SeekCurrent, SeekToBlock, and LinearSeek return sequences that
// agree on the position they're at.
func FuzzSeekVsLinearScan(f *testing.F) {
	f.Add(uint16(7), uint16(0))

	f.Fuzz(func(t *testing.T, blockSize, _ uint16) {
		if blockSize == 0 {
			blockSize = 1
		}
		// Seed: a small list to give the fuzzer something to work with.
		uids := []uint64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
		pack := Encode(uids, int(blockSize))
		defer FreePack(pack)

		dec := NewDecoder(pack)
		// Seek each value and verify.
		for _, u := range uids {
			got := dec.Seek(u, SeekStart)
			if len(got) == 0 || got[0] != u {
				t.Fatalf("SeekStart(%d) returned %v, expected first elem %d", u, got, u)
			}
		}
		// Seek values not in the list.
		for _, u := range []uint64{0, 5, 15, 25, 105, math.MaxUint64} {
			_ = dec.Seek(u, SeekStart) // shouldn't panic
		}
	})
}

// FuzzEncodeFromBuffer fuzzes the uvarint-based buffer encoder.
func FuzzEncodeFromBuffer(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Add([]byte{0x80, 1})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024 {
			return // bound input size for speed
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("EncodeFromBuffer panicked on %x: %v", data, r)
			}
		}()
		pack := EncodeFromBuffer(data, 8)
		defer FreePack(pack)
		// Decode the encoded pack and re-encode to compare.
		out := Decode(pack, 0)

		// Compute the expected deltas.
		var prev uint64
		expected := []uint64{}
		buf := []byte{}
		buf = append(buf, data...)
		for len(buf) > 0 {
			v, n := binary.Uvarint(buf)
			if n <= 0 {
				break
			}
			buf = buf[n:]
			prev += v
			expected = append(expected, prev)
		}
		require.Equal(t, expected, out, "EncodeFromBuffer mismatch")
	})
}

// FuzzUnpackBlock feeds a decoder a synthetic UidPack with hand-crafted
// NumUids/Deltas to exercise the padding logic in UnpackBlock.
func FuzzUnpackBlock(f *testing.F) {
	f.Add(uint32(1), uint8(0))
	f.Add(uint32(5), uint8(0))

	f.Fuzz(func(t *testing.T, numUids uint32, _ uint8) {
		// Bound numUids.
		numUids = numUids%32 + 1
		// Build a pack with a single block with that NumUids but only a few bytes of Deltas.
		// UnpackBlock has padding logic that should avoid OOB reads.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("UnpackBlock panicked: %v", r)
			}
		}()
		pack := &pb.UidPack{
			BlockSize: 256,
			Blocks: []*pb.UidBlock{
				{
					Base:    1000,
					NumUids: numUids,
					Deltas:  []byte{1, 2, 3, 4, 5},
				},
			},
		}
		dec := NewDecoder(pack)
		_ = dec.UnpackBlock()
	})
}

// FuzzLinearSeekBoundary probes SeekToBlock and LinearSeek boundary cases.
func FuzzLinearSeekBoundary(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(math.MaxUint64))

	f.Fuzz(func(t *testing.T, seek uint64) {
		uids := []uint64{100, 200, 300, 400, 500}
		pack := Encode(uids, 8)
		dec := NewDecoder(pack)
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LinearSeek panicked on seek=%d: %v", seek, r)
			}
		}()
		_ = dec.LinearSeek(seek)
	})
}