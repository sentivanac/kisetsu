# ADR 0016: Image processing for the library

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Every downloaded file must be checked before it enters the library, and
the library needs a thumbnail, an exact-duplicate key and a perceptual
hash for each image. Basic functionality must not depend on Python.

## Decision

`internal/imaging.Process(io.ReadSeeker)` does everything in Go, using the
standard library and `golang.org/x/image` (WebP decoding, scaling):

1. **SHA-256** of the file, for exact duplicates.
2. **Header check** with `image.DecodeConfig` before decoding:
   - only JPEG, PNG and WebP are accepted, the formats the WebView can show;
   - images over **120 megapixels** are rejected before any pixel is decoded,
     so a small file claiming huge dimensions (a decompression bomb) cannot
     exhaust memory. 8K images (33 MP) and large panoramas fit.
3. **Full decode**; failures report `ErrCorrupt` (truncated or damaged files).
4. **Thumbnail**: longest side 480 px (gallery cells can exceed 400 px), never
   enlarged, JPEG quality 85, transparent areas on black.
5. **dHash** (64-bit difference hash) from the thumbnail. `Similar` uses a
   distance of at most 10 bits, the usual threshold for dHash.

The decoded dimensions are the truth stored in the library, not the ones
the provider reported.

### Performance

Measured on the development machine (i7-10510U) with a 20 MP JPEG:

| Version | Time |
|---|---|
| Catmull-Rom over the full image | 1.8 s |
| Halve first into `NRGBA`, then Catmull-Rom | 1.7 s |
| Halve first into **`RGBA`**, then Catmull-Rom | **0.89 s** |

Profiling showed that the cost was not the filter but the generic,
per-pixel conversion path of `x/image/draw`; it has fast paths only for
`RGBA` destinations, including directly from the `YCbCr` images returned
by the JPEG decoder. Decoding itself takes about 0.32 s and is the floor.
Processing runs in the background after a download, so this is enough.

### Tests

Test images are generated in code (gradients and shapes) except one
64 × 48 WebP in `testdata/` made with Pillow, because Go has no WebP
encoder. Tests cover all formats, aspect ratios, no upscaling,
transparency, unsupported and damaged files, a 2.5-gigapixel PNG header,
and that re-encoding at JPEG quality 25, halving the size or converting to
PNG keeps the dHash within the threshold while a different picture does not.

## Limitations

- EXIF orientation is not applied: a camera photo stored sideways with an
  orientation tag gets a sideways thumbnail. Most provider images are
  already upright; this can be added if it shows up in practice.
- Animated WebP files are decoded as their first frame.
