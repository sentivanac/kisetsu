import {useSyncExternalStore} from 'react';
import {EventsOn} from '../../wailsjs/runtime/runtime';
import {applyProgress, downloadProgressEvent, DownloadProgress, Downloads} from './downloads';

// A single store for all download events, so every component sees the
// same state and the event listener is registered once.
let downloads: Downloads = {};
const listeners = new Set<() => void>();
let unsubscribe: (() => void) | null = null;

function subscribe(listener: () => void): () => void {
    listeners.add(listener);
    if (!unsubscribe) {
        unsubscribe = EventsOn(downloadProgressEvent, (p: DownloadProgress) => {
            downloads = applyProgress(downloads, p);
            listeners.forEach((l) => l());
        });
    }
    return () => {
        listeners.delete(listener);
        if (listeners.size === 0 && unsubscribe) {
            unsubscribe();
            unsubscribe = null;
        }
    };
}

// useDownloads returns the current state of all downloads of this session.
export function useDownloads(): Downloads {
    return useSyncExternalStore(subscribe, () => downloads);
}
