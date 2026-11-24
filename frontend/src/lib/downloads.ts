// Download state for the UI, built from "download:progress" events.

import {BackendError, describeError} from './backend';

export const downloadProgressEvent = 'download:progress';

export type DownloadState =
    | 'queued'
    | 'downloading'
    | 'processing'
    | 'done'
    | 'duplicate'
    | 'failed'
    | 'canceled';

// DownloadProgress mirrors main.DownloadProgress.
export interface DownloadProgress {
    key: string;
    state: DownloadState;
    received: number;
    total: number;
    entryId?: number;
    similarIds?: number[];
    // A fetched copy that can be kept instead of a failed download.
    fetchedId?: number;
    error?: BackendError;
}

export type Downloads = Record<string, DownloadProgress>;

export function isActive(p: DownloadProgress | undefined): boolean {
    return p?.state === 'queued' || p?.state === 'downloading' || p?.state === 'processing';
}

// applyProgress returns the download map with one event applied. Progress
// events do not repeat the expected size, so it is kept from earlier ones.
export function applyProgress(downloads: Downloads, p: DownloadProgress): Downloads {
    const previous = downloads[p.key];
    const total = p.total || previous?.total || 0;
    const received = p.state === 'processing' ? Math.max(p.received, previous?.received ?? 0) : p.received;
    return {...downloads, [p.key]: {...p, total, received}};
}

// percent returns the downloaded share from 0 to 100, or null when the size
// is unknown.
export function percent(p: DownloadProgress): number | null {
    if (p.state === 'processing') return 100;
    if (!p.total || p.total <= 0) return null;
    return Math.min(100, Math.floor((p.received / p.total) * 100));
}

// downloadLabel is the text of the download button.
export function downloadLabel(p: DownloadProgress | undefined, inLibrary: boolean): string {
    if (inLibrary || p?.state === 'done' || p?.state === 'duplicate') return 'In your library';
    switch (p?.state) {
        case 'queued':
            return 'Waiting…';
        case 'downloading': {
            const pct = percent(p);
            return pct === null ? 'Downloading…' : `Downloading ${pct}%`;
        }
        case 'processing':
            return 'Checking image…';
        case 'failed':
            return 'Try again';
    }
    return 'Download';
}

// downloadMessage explains the outcome of a finished download, if needed.
export function downloadMessage(p: DownloadProgress | undefined): string | null {
    if (!p) return null;
    if (p.state === 'failed' && p.error) return describeError(p.error);
    if (p.state === 'duplicate') return 'An identical file was already in your library.';
    const similar = p.similarIds?.length ?? 0;
    if (p.state === 'done' && similar > 0) {
        return similar === 1
            ? 'Saved. It looks like an image already in your library.'
            : `Saved. It looks like ${similar} images already in your library.`;
    }
    if (p.state === 'done') return 'Saved to your library.';
    return null;
}

export type DownloadSize = 'original' | 'screen';

export interface Size {
    width: number;
    height: number;
}

// screenCopySize is the size of a copy of img reduced to cover the screen,
// or null when img is not larger than that (it is then stored as it is).
// The same rule as coverSize in internal/imaging.
export function screenCopySize(img: Size, screen: Size): Size | null {
    if (img.width <= 0 || img.height <= 0 || screen.width <= 0 || screen.height <= 0) return null;
    const scale = Math.max(screen.width / img.width, screen.height / img.height);
    if (scale >= 1) return null;
    return {
        width: Math.max(screen.width, Math.ceil(img.width * scale)),
        height: Math.max(screen.height, Math.ceil(img.height * scale)),
    };
}

// DownloadChoice is one version of an image that can be downloaded.
export interface DownloadChoice {
    size: DownloadSize;
    label: string; // "Original · 7360 × 4912"
}

// downloadChoices returns the versions to offer, the preferred one first.
// Without an original larger than the screen there is nothing to choose.
// original is null when its size is not known.
export function downloadChoices(
    original: Size | null,
    screenCopy: Size | null,
    preferred: DownloadSize,
): DownloadChoice[] {
    const text = (s: Size) => `${s.width} × ${s.height}`;
    const first: DownloadChoice = {
        size: 'original',
        label: original ? `Original · ${text(original)}` : 'Original',
    };
    if (!screenCopy) return [first];
    const second: DownloadChoice = {size: 'screen', label: `Screen size · ${text(screenCopy)}`};
    return preferred === 'screen' ? [second, first] : [first, second];
}
