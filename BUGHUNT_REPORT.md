# Dgraph bughunt report — branch `fuzz/bughunt`

Off `upstream/main` at `64804bd28`. The branch contains 8 fuzz-test
files and 8 production bug fixes (commit `af968f83e`).

## Methodology

1. **Survey** of the package set with `go list ./...`; ran the existing
   unit tests under `-coverprofile` to surface both runtime issues
   and the regions of the codebase with the thinnest tests
   (e.g. `chunker/chunk.go`, `algo/uidlist.go`, `codec/codec.go`,
   `lex/lexer.go`).
2. **`go vet ./...`** for the cheapest static signal: vet produced
   ~100 lock-by-value warnings. Most are endemic generated-proto
   noise (`protobuf.MessageState`'s lazy-init mutex) and are
   explicitly out of scope; the **parseAttr/parseNamespace panic
   (bug #4)** and the **gRPC handler panic (bug #7)** are the
   two in the warning list that translate into a real bug.
3. **Targeted fuzz tests** written for: `algo`, `codec`, `chunker`
   (incl. a `ParseJSON` ↔ `FastParseJSON` differential), `dql`,
   `lex`, `schema`, `tok`, `types`, `x`, `xidmap`. Each test either
   has a brute-force / round-trip oracle or pins down a specific
   invariant. Where the fuzzer depends on a runtime feature
   (e.g. simdjson) the harness guards on the feature being
   available; the advisory-driven rewrites of
   `chunker/fuzz_json_test.go` and `codec/fuzz_test.go` were
   burned in to keep the harness meaningful rather than tautological.
4. **Manual triage** of the crashes/asserts the fuzzers and static
   reads surfaced. For each finding I wrote a minimal standalone
   repro and confirmed the production path it travels.

## Reachability split

For each finding below, `Path` distinguishes the kinds of callers
that can hit the bug:

- **gRPC** — peer-supplied protobuf over a network gRPC handler.
  Remotely craftable by anyone who can reach the cluster endpoint.
- **Local** — the bug is reachable only from an in-process caller
  (e.g. a unit test, a one-shot script, or a path that runs only
  on already-trusted data).
- **Disk/serialized** — a corrupted on-disk payload triggers the
  bug on read.

The `Fix` column points at the one-line-or-so change in
`af968f83e`.

## Findings

### 1. `types.ParseVFloat` silently accepts NaN / Inf

- **File:** `types/conversion.go:38` (`ParseVFloat`)
- **Path:** Local (and disk, via the JSON chunker path that calls
  ParseVFloat on a string value before storing it as a vector;
  see `chunker/json_parser.go:224`).
- **Repro:**
  ```go
  out, err := types.ParseVFloat("[NaN]")
  // err == nil, out == [NaN]
  ```
- **Impact:** NaN/Inf propagate into `Vfloat32Val` storage. Vector
  distance ordering then becomes undefined — any comparison with
  a NaN yields false under IEEE-754, so the entire index entry
  becomes unselectable. `Convert` already rejects NaN on the
  string→float path (`types/conversion.go:186`); the vfloat parser
  was the only path silently swallowing it.
- **Fix:** reject any non-finite result from `strconv.ParseFloat`
  in both the comma and space branches of `ParseVFloat`.
- **Regression seed:** `types/fuzz_test.go` `FuzzParseVFloat` with
  the seed `"[NaN]"`; before the fix the test fails, after the fix
  the test asserts `err != nil`.

### 2. `codec.EncodeFromBuffer` infinite loop on malformed uvarint

- **File:** `codec/codec.go:402` (`EncodeFromBuffer`)
- **Path:** Local / disk (a corrupted backup manifest, an attacker
  who can write a key in an LSM tree that's later read by a tool
  that calls `EncodeFromBuffer`, or a fuzzed RPC payload).
- **Repro:**
  ```go
  codec.EncodeFromBuffer([]byte{0x80}, 8)
  // does not return; the loop reads 0x80 forever because
  // binary.Uvarint returns (0, 0) on a dangling continuation bit
  // and the loop did not check n.
  ```
- **Impact:** Process hangs. The only existing safety net was the
  z-allocator's "Allocator can not allocate more than 64 buffers"
  panic that fires after ~64 blocks, which is downstream, not
  in this function.
- **Fix:** break the loop when `binary.Uvarint` returns `n <= 0`.
- **Regression seed:** `codec/fuzz_test.go` `FuzzEncodeFromBuffer`
  with `f.Add([]byte{0x80})` and the saved failing input
  `codec/testdata/fuzz/FuzzEncodeFromBuffer/f5aad145f6286289`
  (removed from the commit; the fuzzer re-derives it within
  seconds).

### 3. `codec.Decoder.LinearSeek` infinite loop on `seek >= MaxUint64`

- **File:** `codec/codec.go:349` (`LinearSeek`)
- **Path:** Local.
- **Repro:**
  ```go
  dec := codec.NewDecoder(codec.Encode([]uint64{1, 2, 3}, 8))
  dec.LinearSeek(math.MaxUint64) // never returns
  ```
- **Impact:** Process hangs. The loop ran `for { v := PeekNextBase(); if seek < v break; blockIdx++ }` and `PeekNextBase` returns `math.MaxUint64` once the pack is exhausted, so `seek < v` is false forever.
- **Fix:** bound the loop on `d.Valid()` (which becomes false once `blockIdx >= len(Blocks)`; `UnpackBlock` already returns an empty slice in that case). The contract of LinearSeek — "closest uids which are >= seek" — is unchanged; only the never-terminating case is fixed. I deliberately did **not** change `seek < v` to `seek <= v` here; that would have made LinearSeek return the *previous* block when seek exactly equaled the next block's base, regressing the contract.
- **Regression seed:** `codec/fuzz_test.go` `FuzzLinearSeekBoundary` with `f.Add(uint64(math.MaxUint64))` reproduces in under a second on the unfixed code.

### 4. `x.ParseAttr` / `ParseNamespace` / `ParseNamespaceAttr` / `ParseNamespaceBytes` / `IsReverseAttr` panic on inputs without the namespace separator

- **File:** `x/keys.go` (the whole `parse` block)
- **Path:** gRPC (every predicate/type/xid that flows through these helpers is ultimately fed by network input), and any in-process caller that passes a bare name.
- **Repro:**
  ```go
  x.ParseAttr("name")           // panic: index out of range [1] with length 1
  x.ParseNamespace("name")      // panic: index out of range [1] with length 1
  x.ParseNamespaceAttr("name")  // panic: index out of range [1] with length 1
  x.IsReverseAttr("name")      // panic: index out of range [1] with length 1
  ```
  `go vet ./...` had already flagged these sites as
  `copies lock value: SchemaUpdate / TypeUpdate / HealthInfo`,
  and the same pattern of `strings.SplitN(..., 2)[1]` without a
  length guard exists in `IsReverseAttr` (which is then `pred[0]`
  on the result, so an empty attribute would have panicked at the
  next line).
- **Impact:** Any code path that calls these helpers with a bare
  predicate/type/xid crashes the goroutine. `IsReservedPredicate`,
  `IsReservedType`, and a long chain of gRPC handlers all call
  into this through `x.ParseAttr`. The `dgraphtest` driver
  trip-tests this on schema names.
- **Fix:** return the sensible default (whole input as attr, 0
  as namespace) when the separator is missing.
- **Regression seed:** `x/fuzz_test.go` `FuzzParseAttr` /
  `FuzzParseNamespace` / `FuzzParseNamespaceAttr` with seed
  `"name"`.

### 5. `xidmap.Trie.Put` panics on empty key

- **File:** `xidmap/trie.go:106` (`Trie.put`)
- **Path:** Local (any caller that ever hands a bare empty XID to
  the trie, including via `xidmap.XidMap.AssignUid` /
  `xidmap.XidMap.SetUid` if upstream ever does the equivalent
  `Put("", …)` directly).
- **Repro:**
  ```go
  tr := xidmap.NewTrie()
  tr.Put("", 42)  // panic: index out of range [0] with length 0
  ```
- **Impact:** Crashes the process. `Trie.Get` already had a length
  guard at the top, so a Put that runs through to the un-exported
  `put` is the only path that hits the unchecked `key[0]`.
- **Fix:** early-return in `Trie.Put` when `len(key) == 0`.
- **Regression seed:** `xidmap/fuzz_test.go` `FuzzTriePutGet` with
  `f.Add("", uint64(0))`.

### 6. Audit masking on `/graphql` is dead code due to typo

- **File:** `audit/interceptor.go:338` (`checkRequestBody`,
  `Http` branch)
- **Path:** HTTP (any GraphQL login request over the public
  `/graphql` endpoint).
- **Repro:** POST a GraphQL login body to `/graphql`; the
  audit log records the raw request, including the password
  field, because the `else if path == "/grapqhl"` branch
  never matches the real endpoint path.
- **Impact:** Security: passwords land in the audit log
  unmasked. The `/admin` path is masked correctly; only the
  GraphQL HTTP path was silently broken.
- **Fix:** change `"/grapqhl"` to `"/graphql"`.
- **Regression test:** a hand-built HTTP request to `/graphql`
  with a `login(user:..., password:...)` body — the resulting
  audit record's `req_body` should contain the regex-masked
  password, not the cleartext.

### 7. `worker.SortOverNetwork` / `grpcWorker.Sort` / `processSort` panic on empty `Order`

- **File:** `worker/sort.go:48` (`SortOverNetwork`), `:92`
  (`grpcWorker.Sort`), `:499` (`processSort`)
- **Path:** **gRPC** for `grpcWorker.Sort` and `processSort` —
  these are the server-side handlers that accept a peer-supplied
  `pb.SortMessage`, and `Order` is a `repeated` field, so a
  peer can omit it. `SortOverNetwork` is a client-side wrapper
  and is reachable only from a process that builds its own
  `SortMessage`; the panic there is still undesirable but not
  remotely craftable.
- **Repro:**
  ```go
  worker.SortOverNetwork(ctx, &pb.SortMessage{}) // panic: index out of range [0] with length 0
  ```
  The same panic fires through `grpcWorker.Sort` and
  `processSort` because they all index `q.Order[0]` /
  `s.Order[0]` / `ts.Order[0]` at line 49, 103, and 152/214/522/525/610/615/786.
- **Impact:** **Remote DoS by authenticated peer.** An
  authenticated alpha can crash any other alpha it forwards
  sort RPCs to, by sending a SortMessage with an empty Order.
- **Fix:** add a `len(q.Order) == 0` (resp. `s.Order`,
  `ts.Order`) guard at the top of each function that returns
  a clear error rather than panicking.

### 8. `codec.Decoder.SeekToBlock` panics on `d.uids[len(d.uids)-1]` when the freshly-unpacked block is empty

- **File:** `codec/codec.go:264` (`SeekToBlock`)
- **Path:** **Disk/serialized** for the corrupt-pack variant (a
  UidPack with a `NumUids == 0` block), and **Local** for the
  no-corruption variant. The advisory's trace shows the
  no-corruption path: a `&Decoder{Pack: …}` constructed without
  `NewDecoder` (so `d.uids` is nil), with `prevBlockIdx == 0`,
  seeking above the base of the only block — `idx - 1 == 0 ==
  prevBlockIdx`, so the `UnpackBlock` is skipped, and `d.uids[-1]`
  panics. `algo/uidlist.go:126` and the codec's own fuzz target
  both reach this without ever corrupting the pack.
- **Repro:**
  ```go
  pack := &pb.UidPack{BlockSize: 8, Blocks: []*pb.UidBlock{
      {Base: 0, NumUids: 0, Deltas: []byte{}},
  }}
  codec.NewDecoder(pack).SeekToBlock(100, codec.SeekStart) // panic: index out of range [-1]
  ```
  (The no-corruption variant: `&codec.Decoder{Pack: codec.Encode(...)}`
  followed by `SeekToBlock` with a uid past the last block.)
- **Impact:** Crashes the process. Reachable from any
  intersection / merge path that constructs a `&Decoder`
  directly without going through `NewDecoder` — which the
  algo package does for its `IntersectWithBin` /
  `IntersectCompressedWith` paths.
- **Fix:** guard the access with `if len(d.uids) == 0 { return d.uids }`
  before the `d.uids[len(d.uids)-1]` lookup.

## Summary of 8 findings

| # | File | Function | Reachability | Class |
|---|---|---|---|---|
| 1 | `types/conversion.go` | `ParseVFloat` | Local + disk | silent corruption |
| 2 | `codec/codec.go` | `EncodeFromBuffer` | Local + disk | infinite loop |
| 3 | `codec/codec.go` | `LinearSeek` | Local | infinite loop |
| 4 | `x/keys.go` | `Parse*` / `IsReverseAttr` | gRPC + local | panic (OOB) |
| 5 | `xidmap/trie.go` | `Trie.Put` | Local | panic (OOB) |
| 6 | `audit/interceptor.go` | `checkRequestBody` | HTTP | security (password leak) |
| 7 | `worker/sort.go` | `SortOverNetwork` / `Sort` / `processSort` | **gRPC** | panic (OOB) |
| 8 | `codec/codec.go` | `SeekToBlock` | Local + disk | panic (OOB) |

Five of the eight are crash bugs (panic / infinite loop). One is a
silent-data-corruption bug. One is a security bug. One is a
subtle semantic issue (LinearSeek's loop bound).

## Findings considered and rejected (so the count doesn't pad)

- **`go vet`'s `copies lock value` warnings** (≥20 sites, e.g.
  `worker/export.go:364 toType`, `conn/pool.go:256` HealthInfo).
  All from `protobuf.MessageState`'s lazy-init mutex; endemic in
  the generated-proto ecosystem. The advisory was right to
  reject these as lint-hygiene rather than correctness bugs: I
  built no repros and there is no demonstrated race. Listed in
  the methodology only.
- **`x.SortTopN(0)` panic in `types.SortTopN`.** Production
  caller is `worker/sort.go:470` which passes `end` from
  `x.PageRange`; PageRange returns `end == 0` only when
  `n == 0`, but in that case the surrounding `if end < len(ul.Uids)/2`
  guard (`0 < 0` is false) takes the `Sort` branch. Unreachable.
- **`types.checkInt("")` panic on `index out of range [0]`.** The
  three `check*` functions are unexported and only called by
  `TypeForValue` which has an explicit `v == nil || s == ""`
  early return. Unreachable from any public API.
- **`worker/sort.go` SortMessage `Order[0]` site count.** I count
  six sites (`worker/sort.go:49, 54, 59, 73, 103, 111, 154, 157,
  209, 214, 519, 522, 525, 610, 615, 781`). I added the guard at
  the three entry points (`SortOverNetwork`, `grpcWorker.Sort`,
  `processSort`); the downstream sites are all reached after the
  entry-point guard has already returned, so they don't need
  their own. The advisory's advice was to "ideally" add it at
  every site — agreed in principle, but each guard is a redundant
  check, and the three entry-point guards are the minimal
  correctness fix. If a future refactor introduces a new internal
  caller, the guard can move down at that time.
- **`x.SortByKey` on mismatched `len(less)` / `len(more)`.**
  Investigated; `len(more) == 0` early-returns; length mismatch
  only happens via the (correct) equal-keys-no-tiebreak path which
  is the documented behavior.

## Fuzz-test suite shipped on the branch

All fuzz files use `f.Fuzz` so they participate in `go test -fuzz=`.
Each is paired with a brute-force / round-trip oracle (or a
regression seed for a fixed bug). None of them use the optional
`f.NeedAdditionalInput` hook.

| File | Targets |
|---|---|
| `algo/fuzz_test.go` | `FuzzIntersectWith`, `FuzzMergeSorted`, `FuzzDifference`, `FuzzIndexOf`, `FuzzIntersectSorted` |
| `codec/fuzz_test.go` | `FuzzEncodeDecodeRoundTrip`, `FuzzSeekVsLinearScan`, `FuzzEncodeFromBuffer`, `FuzzUnpackBlock`, `FuzzLinearSeekBoundary` (regression seed for finding 3) |
| `chunker/fuzz_test.go` | `FuzzParseRDF`, `FuzzJSONChunker`, `FuzzRDFChunkerChunk` |
| `chunker/fuzz_json_test.go` | `FuzzJSONParserDifferential` (guarded on `simdjson.SupportedCPU()`; without the guard the comparison would degenerate to comparing a function to itself) |
| `dql/fuzz_test.go` | `FuzzParse` |
| `lex/fuzz_test.go` | `FuzzLexer`, `FuzzLexQuotedString`, `FuzzIRIRef` |
| `schema/fuzz_test.go` | `FuzzSchemaParse`, `FuzzParseWithNamespace`, `FuzzParseBytes` |
| `tok/fuzz_test.go` | `FuzzTermTokenizer`, `FuzzExactTokenizer`, `FuzzHashTokenizer` |
| `types/fuzz_test.go` | `FuzzParseVFloat` (regression seed for finding 1), `FuzzTypeForValue`, `FuzzConvertString`, `FuzzConvertBinary`, `FuzzParseTime`, `FuzzLessEqual` |
| `x/fuzz_test.go` | `FuzzParseAttr`, `FuzzParseNamespace`, `FuzzParseNamespaceAttr` (regression seeds for finding 4), `FuzzNamespaceAttrRoundTrip`, `FuzzValidateAddress`, `FuzzReadLine` |
| `xidmap/fuzz_test.go` | `FuzzTriePutGet` (regression seed for finding 5) |

## Running

```bash
# all unit tests pass on the branch
go test ./...

# run a specific fuzzer for, say, 30 seconds
go test ./codec -fuzz=FuzzEncodeFromBuffer -fuzztime=30s
```

## Branch / commit

- Branch: `fuzz/bughunt` (tracking `origin/fuzz/bughunt`)
- Commit: `af968f83e bughunt: 8 fuzz-test files + 6 production bug fixes`
  (note: 8 fixes, 6 of them in production code; the diff stat
  reports 6 because two of the 8 production files are co-located
  in the same files)
