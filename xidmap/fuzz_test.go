/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package xidmap

import (
	"testing"
)

// FuzzTriePutGet exercises Trie Put and Get against arbitrary inputs.
func FuzzTriePutGet(f *testing.F) {
	f.Add("", uint64(0))
	f.Add("a", uint64(1))
	f.Add("hello", uint64(42))
	f.Add("a\x00b", uint64(7))
	f.Add("\xff\xff", uint64(99))

	f.Fuzz(func(t *testing.T, key string, uid uint64) {
		if len(key) > 1024 {
			return // bound input size
		}
		tr := NewTrie()
		defer tr.Release()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Trie Put/Get panicked on key=%q uid=%d: %v", key, uid, r)
			}
		}()
		tr.Put(key, uid)
		// The Get for non-empty keys should recover the value we just put.
		if key != "" {
			got := tr.Get(key)
			if got != uid {
				t.Fatalf("Trie Get mismatch: key=%q want=%d got=%d", key, uid, got)
			}
		}
	})
}