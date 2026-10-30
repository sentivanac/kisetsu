import {BackendError, describeError} from '../lib/backend';
import {DownloadProgress, downloadLabel, downloadMessage, isActive, percent} from '../lib/downloads';

interface Props {
    progress?: DownloadProgress;
    inLibrary: boolean;
    // An error from starting the download (before any progress event).
    startError?: BackendError;
    onDownload: () => void;
    onCancel: () => void;
}

// DownloadButton downloads one image into the library and shows progress.
export function DownloadButton({progress, inLibrary, startError, onDownload, onCancel}: Props) {
    const active = isActive(progress);
    const saved = inLibrary || progress?.state === 'done' || progress?.state === 'duplicate';
    const pct = progress && active ? percent(progress) : null;
    const message = startError ? describeError(startError) : downloadMessage(progress);
    const failed = Boolean(startError) || progress?.state === 'failed';

    return (
        <div className="download">
            <div className="download-row">
                <button type="button" className="btn" onClick={onDownload} disabled={active || saved}>
                    {saved ? '✓ ' : ''}
                    {downloadLabel(progress, inLibrary)}
                </button>
                {active && (
                    <button type="button" className="btn secondary" onClick={onCancel}>
                        Cancel
                    </button>
                )}
            </div>
            {active && (
                <progress
                    className="download-progress"
                    max={100}
                    value={pct ?? undefined}
                    aria-label="Download progress"
                />
            )}
            {message && (
                <p className={failed ? 'notice error' : 'notice'} role={failed ? 'alert' : 'status'}>
                    {message}
                </p>
            )}
        </div>
    );
}
