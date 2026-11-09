# ADR 0052: Scaling images in little memory

- **Status:** accepted
- **Date:** 2026-11-09

## Context

The agent on Windows used about 40 MB, as expected. On Linux, with a
library image larger than the screen, it used 152 MB, and Go's heap
reached 85 MB while composing the wallpaper (ADR 0047).

The cause was the last scaling step, `draw.CatmullRom` from
`golang.org/x/image`: it filters along the rows into a temporary buffer
of `[4]float64`, 32 bytes per pixel of (target width × source height),
and keeps the buffer in a pool for the next call. Measured with
`BenchmarkResize` on an Intel i7-10510U:

| Scaling | Time | Memory |
| --- | --- | --- |
| 2560×1600 → 1400×875 | 0.55 s | 77 MB |
| 6000×4000 → 1920×1280 | 3.7 s | 256 MB |
| 6000×4000 → 3840×2560 | 7.3 s | 532 MB |

The same scaling serves downloads (images fitted to the screen), the
images fetched for rotation, and the agent, which runs all session.

## Decision

1. **Own Catmull-Rom scaler** (`internal/imaging/resample.go`), used for
   the last step of `imaging.Resize`. It filters along the rows one
   source row at a time and keeps only the rows the filter needs for the
   next output row (at most 4 × the scale factor, plus one), in `float32`.
   The filter is widened by the scale factor when shrinking, as in
   `x/image`. Halving the image first while it is at least twice the
   target size (2×2 averages) stays as it was.
2. **The agent returns memory after each composed image**
   (`onscreen.Service.ReleaseMemory`, `debug.FreeOSMemory`): between
   redraws it needs only the kept base. A forced collection takes a few
   milliseconds once a minute at most. The window does not do this.

The difference hash of duplicate detection still uses `x/image`: it
scales to 9×8 pixels (about 1 MB), and a different scaler would change
the hashes stored in existing libraries.

## Measured

Same benchmark, after:

| Scaling | Time | Memory |
| --- | --- | --- |
| 2560×1600 → 1400×875 | 0.37 s | 6 MB |
| 6000×4000 → 1920×1280 | 2.0 s | 12 MB |
| 6000×4000 → 3840×2560 | 2.4 s | 42 MB (39 MB of it the result) |

The result differs from `x/image`'s on average by 0.09 to 0.17 of 255
per channel on test images (shrinking, enlarging, almost the same
size), and is the same at the same size. On photos from a library: 0.18
and 0.20 for a 7360×4912 photo scaled to 2560 and 3840 px wide, 0.21
for a 1921×1276 one to 1920 px; invisible.

The agent in a test session (1400×900 screen, a 2560×1600 image, the
text file and all effects on): 109 MB of its own memory before, 16 to
19 MB after, also after repeated redraws; at its peak, while composing,
about 40 MB. The rest of its resident memory (about 42 MB) is the mapped
program and libraries, shared with other processes.

## Alternatives considered

- **Bilinear scaling for the last step** (`draw.ApproxBiLinear`, no
  buffer): visibly softer and with more aliasing than Catmull-Rom.
- **Scaling in strips with `x/image`:** the strips' edges would not line
  up exactly, leaving seams.
- **Only freeing memory after composing:** the peak of hundreds of MB on
  a 4K screen would stay.
