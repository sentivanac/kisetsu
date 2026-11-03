# ADR 0035: Performance with a large library

- **Status:** accepted
- **Date:** 2026-11-03
- Phase 7, "Improve performance".

## Context

Measured before changing anything: Go benchmarks
(`internal/library/library_bench_test.go`) with a library of **10 000
images** — far more than most users keep — on the development machine
(i7-10510U, SSD).

## Results

| Operation | When it runs | Before | After |
|---|---|---|---|
| One Library page (60 images, middle of the library) | opening the tab, "Load more" | **140 ms** | **2.3 ms** |
| Rotation candidates (all library IDs) | every rotation | 28 ms | 9 ms |
| Library count and size | Library tab header | 9 ms | 9 ms |
| Duplicate check (read all dHashes, compare) | after every download | 18 ms | 18 ms |

## Decision

Only the library page was too slow. Its query (`WHERE origin = ? ORDER
BY added_at DESC, id DESC LIMIT … OFFSET …`) used the `(origin,
saved_search_id)` index and then sorted every image of that origin
(`USE TEMP B-TREE FOR ORDER BY`) to return 60.

Migration **0008** adds the index `images_origin_added (origin, added_at,
id)`. The rows now come out of the index in order, without sorting; the
rotation's `IDs` query (`ORDER BY added_at, id`) uses it too. Existing
databases get it at the next start.

The other operations stay as they are: they are well under the 50 ms a
user would notice, and grow linearly (the duplicate check about 2 ms per
1 000 images).

Not benchmarked separately: `rotation.Pick` works on the ID list in
memory, and `History.State` reads one cycle through its index
(`wallpaper_history_cycle`).

## Verification

`go test -run '^$' -bench . -benchmem ./internal/library/` (seeding the
10 000 images takes about a minute). Query plans compared with
`EXPLAIN QUERY PLAN` before and after the index.
