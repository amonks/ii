# Performance Audit

## Scope

- Focus on `ii todo` flows and the `todo` package first.
- Keep benchmark data, profiling commands, and notable improvements here.

## Benchmark setup

- Command (JSONL read/write): `go test ./todo -bench='(ReadJSONLFromReader|WriteJSONL)' -run=^$ -benchmem`
- Command (store operations): `go test ./todo -bench='Store(Create|DepAdd|DepTree|List|Ready|Show|Update)' -run=^$ -benchmem`
- Environment: darwin/arm64 (Apple M1 Ultra)
- Benchmark data: JSONL payload synthesized in-memory by `BenchmarkReadJSONLFromReader*`.

## Measurements

2026-01-25

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| `BenchmarkReadJSONLFromReader1K` | 1,697,588 | 477,385 | 9,004 |
| `BenchmarkReadJSONLFromReader10K` | 17,137,010 | 4,835,435 | 90,008 |
| `BenchmarkWriteJSONL1K` | 431,026 | 888 | 9 |
| `BenchmarkWriteJSONL10K` | 2,582,216 | 1,046 | 9 |
| `BenchmarkStoreList1K` | 1,754,397 | 666,991 | 9,011 |
| `BenchmarkStoreList10K` | 17,824,188 | 6,679,559 | 90,015 |
| `BenchmarkStoreReady1K` | 2,025,794 | 782,023 | 10,532 |
| `BenchmarkStoreReady10K` | 20,403,612 | 7,746,903 | 105,051 |
| `BenchmarkStoreReadyLimit10K` | 20,083,293 | 5,841,622 | 105,057 |
| `BenchmarkStoreShow1K` | 893,007 | 242,671 | 5,022 |
| `BenchmarkStoreShow10K` | 8,796,334 | 2,438,680 | 50,024 |
| `BenchmarkStoreCreate1K` | 2,231,664 | 487,506 | 9,033 |
| `BenchmarkStoreCreate10K` | 20,189,646 | 4,891,559 | 90,041 |
| `BenchmarkStoreDepAdd1K` | 2,383,036 | 559,183 | 10,537 |
| `BenchmarkStoreDepAdd10K` | 20,686,995 | 5,613,685 | 105,045 |
| `BenchmarkStoreDepTree1K` | 2,892,055 | 1,306,255 | 18,054 |
| `BenchmarkStoreDepTree10K` | 30,585,996 | 12,181,468 | 180,237 |
| `BenchmarkStoreUpdate1K` | 2,223,664 | 487,332 | 9,027 |
| `BenchmarkStoreUpdate10K` | 20,181,942 | 4,907,458 | 90,035 |

## Improvements log

- 2026-01-25: Reused the locked file descriptor for JSONL reads to avoid extra open/close syscalls during store reads.
- 2026-01-25: Replaced streaming JSONL decoding with a buffered line reader to preserve one-object-per-line semantics and enforce the max JSON line size guard deterministically.
- 2026-01-25: Avoided copying when a JSONL line fits in the buffered reader by returning the underlying slice, reducing allocations per line.
- 2026-01-25: Buffered JSONL writes before renaming the temp file, trimming syscall overhead and improving write throughput.
- 2026-01-25: Added benchmarks for store-level list/ready operations to track end-to-end todo command costs beyond JSONL parsing.
- 2026-01-25: Estimated JSONL item counts from reader sizes to preallocate slices, cutting bytes/op for reads and store queries.
- 2026-01-25: Preallocated list/ready result slices and blocker maps so store queries avoid repeated growth allocations when scanning todos.
- 2026-01-25: Added a ready-limit benchmark and switched ready queries with limits to a heap selection pass before sorting, keeping ranking consistent while avoiding full sorts for small limits.
- 2026-01-25: Increased JSONL read/write buffer sizes to 64 KiB to reduce syscall overhead during line reads and batched writes.
- 2026-01-25: When Ready runs with a limit, select into the heap while scanning instead of building the full ready slice, trimming bytes/op for ready-limit benchmarks.
- 2026-01-25: Disabled HTML escaping in JSONL writes to shave overhead from JSON encoding while preserving valid output.
- 2026-01-25: Resolve dependency blocker statuses by the dependency ID set instead of mapping every todo, cutting Ready bytes/op by avoiding a full todo lookup map.
- 2026-01-25: Switched JSONL line reads to bufio.ReadLine to avoid extra newline handling work while keeping the max line size guard intact.
- 2026-01-25: Dropped redundant JSONL line-ending trimming now that bufio.ReadLine already strips terminators, reducing per-line work during JSONL reads.
- 2026-01-25: Unmarshal JSONL data directly into the destination slice slot to avoid copying each decoded item, trimming allocations across read and store benchmarks.
- 2026-01-25: Added custom JSONL encoders for todos and dependencies to avoid per-field time.Time MarshalJSON allocations during writes.
- 2026-01-25: Reused scratch buffers while encoding JSONL writes so each item avoids fresh allocations, cutting write allocations and improving throughput.
- 2026-01-25: Reused the todo list/ready in-memory results to compute ID prefix lengths in `ii todo`, eliminating redundant JSONL reads for list/ready output.
- 2026-01-25: Added dependency tree benchmarks to track `ii todo dep tree` performance at scale.
- 2026-01-25: Added store update benchmarks to track update/write costs alongside read-heavy list/ready operations.
- 2026-01-25: Added store show benchmarks to track `ii todo show` costs alongside other read-only commands.
- 2026-01-25: Added store create benchmarks to measure the read-modify-write cost of `ii todo create` for 1K/10K datasets.
- 2026-01-25: Added StoreDepAdd benchmarks to quantify dependency add read/modify/write costs for todo workloads.
- 2026-01-25: Reused the JSONL line buffer when assembling oversized lines so multi-chunk reads avoid repeated allocations.
- 2026-01-25: Preallocated dependency maps and per-node children slices when building dep trees to reduce allocation churn during dep tree queries.
- 2026-01-25: Preallocated dep-tree dependency slices when grouping dependencies by todo, trimming allocation growth during dep tree traversal.
- 2026-01-25: Pooled JSONL reader buffers to reuse the 64 KiB read and line scratch buffers across reads, reducing read allocations and downstream store bytes/op.
- 2026-01-25: Pooled JSONL writer buffers so write paths reuse the 64 KiB buffer instead of allocating a fresh bufio.Writer each time.
- 2026-01-25: Removed the redundant seek before reading locked JSONL files so store reads avoid an extra syscall per file.
- 2026-01-25: Reused normalized todo IDs for prefix length and prefix matching to avoid lowercasing work when resolving IDs.
- 2026-01-25: Avoided lowercasing already-normalized IDs in `ids.NormalizeUniqueIDs` to reduce allocations when building ID indexes.
- 2026-01-25: Compute unique prefix lengths by sorting IDs and comparing neighbors to avoid quadratic scans when rendering todo ID prefixes.
- 2026-01-25: Track blocker resolution states in a single map for Ready to reduce map allocations while keeping missing blockers non-blocking.
- 2026-01-25: Preallocated Update's ID set and updated slice to avoid repeated growth while applying todo updates.
- 2026-01-25: Skip building a full ID index when resolving already-normalized full-length IDs, reducing Update allocations for common cases.
- 2026-01-25: Reused the missing-ID map for exact ID resolution so Update avoids allocating a duplicate map when checking for missing todos.
- 2026-01-25: Built the Show todo lookup map only for requested IDs to avoid allocating a full todo ID map on every show command.
- 2026-01-25: Pooled JSONL write line buffers for todos and dependencies so write operations reuse scratch slices and avoid per-call allocations.
- 2026-01-25: Fast-pathed single-ID Show and Update requests to avoid building maps when only one todo is requested.
- 2026-01-25: Streamed exact-ID show reads so Show can return matching todos without allocating the full todo slice when full-length IDs are provided.

## Profiling notes

- 2026-01-25 (BenchmarkReadJSONLFromReader10K): CPU profile dominated by runtime.madvise and scheduler/GC work, suggesting allocations are the primary cost driver.
- 2026-01-25 (BenchmarkReadJSONLFromReader10K): Heap profile allocations concentrate in readJSONLFromReader/readJSONLLine buffering and encoding/json.Unmarshal.
- 2026-01-25 (BenchmarkStoreList10K): CPU profile shows syscall/syscall and bufio.Reader.ReadSlice at the top, indicating file I/O and buffering overhead dominate list queries.
- 2026-01-25 (BenchmarkStoreList10K): Heap profile is mostly readJSONLFromReader and encoding/json.Unmarshal allocations while loading todos, with list filtering contributing minimal alloc space.
- 2026-01-25 (BenchmarkStoreReady10K): CPU profile again centers on syscall overhead and buffered reads, matching list workloads.
- 2026-01-25 (BenchmarkStoreReady10K): Heap allocations come from readJSONLFromReader for todos/dependencies plus JSON decoding, with dependency blocker maps contributing the next largest share.
- 2026-01-25 (BenchmarkStoreReadyLimit10K): CPU profile still dominated by syscall/syscall with json decoding work next, so Ready-limit performance remains bound by file I/O.
- 2026-01-25 (BenchmarkStoreReadyLimit10K): Heap allocations mostly come from readJSONLFromReader and encoding/json.Unmarshal, with dependency blocker maps contributing the next largest share.
- 2026-01-25 (BenchmarkWriteJSONL10K): CPU profile dominated by syscall.syscall while appendTodoJSONLine and bufio.Writer.Write account for the remaining on-CPU time, confirming file I/O remains the hot path.
- 2026-01-25 (BenchmarkWriteJSONL10K): Heap allocations mainly come from the buffered writer setup and benchmark data generation, with no per-item JSON encoding allocations showing up in the profile.
- 2026-01-25 (BenchmarkStoreDepTree10K): CPU profile still dominated by syscall.syscall and runtime.madvise, with JSON decoding work next, so dependency tree queries remain file I/O bound.
- 2026-01-25 (BenchmarkStoreDepTree10K): Heap allocations are led by JSONL reads and encoding/json.Unmarshal, with buildDepTree and ID normalization contributing the next largest shares.
- 2026-01-25 (BenchmarkStoreDepAdd10K): CPU profile dominated by syscall.syscall with bufio.Reader.ReadLine and bufio.Writer.Write/Flush next, indicating dep add costs are still largely file I/O bound.
- 2026-01-25 (BenchmarkStoreDepAdd10K): Heap allocations concentrate in readJSONLFromReader and encoding/json.Unmarshal while reading todos and dependencies, with buffered writer setup the next largest bucket.
- 2026-01-25 (BenchmarkStoreUpdate10K): CPU profile dominated by syscall.syscall and runtime.madvise, with JSON decoding work next, indicating update costs are still bound by file I/O.
- 2026-01-25 (BenchmarkStoreUpdate10K): Heap allocations concentrate in JSONL reads, encoding/json.Unmarshal, and ID normalization/index building, with buffered writer setup also contributing.
- 2026-01-25 (BenchmarkStoreShow10K): CPU profile dominated by syscall.syscall and bufio.Reader.ReadLine, with JSON decoding and time parsing next, confirming show queries remain file I/O bound.
- 2026-01-25 (BenchmarkStoreShow10K): Heap allocations are led by JSONL reads and encoding/json.Unmarshal, with todo ID map construction the next largest share.
- 2026-01-25 (BenchmarkStoreCreate10K): CPU profile dominated by syscall.syscall, with bufio.Reader.ReadLine and bufio.Writer.Flush/Write next, showing create remains I/O bound beyond the JSON decode/encode work.
- 2026-01-25 (BenchmarkStoreCreate10K): Heap allocations center on readJSONLFromReader and encoding/json.Unmarshal, with benchmark data generation and ID creation providing the next largest allocation buckets.

## Profiling commands

- CPU profile: `go test ./todo -bench=ReadJSONLFromReader10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-read-jsonl.pprof`
- Heap profile: `go test ./todo -bench=ReadJSONLFromReader10K -run=^$ -benchmem -memprofile /tmp/ii-todo-read-jsonl.mem.pprof`
- CPU profile (store list): `go test ./todo -bench=StoreList10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-list.pprof`
- Heap profile (store list): `go test ./todo -bench=StoreList10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-list.mem.pprof`
- CPU profile (store ready): `go test ./todo -bench=StoreReady10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-ready.pprof`
- Heap profile (store ready): `go test ./todo -bench=StoreReady10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-ready.mem.pprof`
- CPU profile (store ready limit): `go test ./todo -bench=StoreReadyLimit10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-ready-limit.pprof`
- Heap profile (store ready limit): `go test ./todo -bench=StoreReadyLimit10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-ready-limit.mem.pprof`
- CPU profile (write JSONL): `go test ./todo -bench=WriteJSONL10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-write-jsonl.pprof`
- Heap profile (write JSONL): `go test ./todo -bench=WriteJSONL10K -run=^$ -benchmem -memprofile /tmp/ii-todo-write-jsonl.mem.pprof`
- CPU profile (store dep tree): `go test ./todo -bench=StoreDepTree10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-deptree.pprof`
- Heap profile (store dep tree): `go test ./todo -bench=StoreDepTree10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-deptree.mem.pprof`
- CPU profile (store dep add): `go test ./todo -bench=StoreDepAdd10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-dep-add.pprof`
- Heap profile (store dep add): `go test ./todo -bench=StoreDepAdd10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-dep-add.mem.pprof`
- CPU profile (store update): `go test ./todo -bench=StoreUpdate10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-update.pprof`
- Heap profile (store update): `go test ./todo -bench=StoreUpdate10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-update.mem.pprof`
- CPU profile (store show): `go test ./todo -bench=StoreShow10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-show.pprof`
- Heap profile (store show): `go test ./todo -bench=StoreShow10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-show.mem.pprof`
- CPU profile (store create): `go test ./todo -bench=StoreCreate10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-create.pprof`
- Heap profile (store create): `go test ./todo -bench=StoreCreate10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-create.mem.pprof`
- Explore: `go tool pprof -http :0 /tmp/ii-todo-read-jsonl.pprof`
