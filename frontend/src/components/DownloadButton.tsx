import {BackendError, describeError} from '../lib/backend';
import {
    DownloadChoice,
    DownloadProgress,
    DownloadSize,
    downloadLabel,
    downloadMessage,
    isActive,
    percent,
} from '../lib/downloads';

interface Props {
    progress?: DownloadProgress;
    inLibrary: boolean;
    // An error from starting the download (before any progress event).
    startError?: BackendError;
    // The versions offered, the preferred one first. Empty while loading.
    choices: DownloadChoice[];
    // The label before a download; "Download" unless set.
    idleLabel?: string;
    onDownload: (size: DownloadSize) => void;
    onCancel: () => void;
    // Keeps the fetched copy of an image whose download failed.
    onKeepCopy?: (fetchedId: number) => void;
}

// DownloadButton downloads one image into the library and shows progress.
export function DownloadButton({
    progress,
    inLibrary,
    startError,
    choices,
    idleLabel,
    onDownload,
    onCancel,
    onKeepCopy,
}: Props) {
    const active = isActive(progress);
    const saved = inLibrary || progress?.state === 'done' || progress?.state === 'duplicate';
    const pct = progress && active ? percent(progress) : null;
    const failed = !inLibrary && (Boolean(startError) || progress?.state === 'failed');
    const message = startError && failed ? describeError(startError) : downloadMessage(progress);
    const [preferred, other] = choices;
    const idle = !progress || progress.state === 'canceled';
    const fetchedId = failed ? progress?.fetchedId : undefined;

    return (
        <div className="download">
            <div className="download-row">
                <button
                    type="button"
                    className="btn"
                    onClick={() => onDownload(preferred.size)}
                    disabled={active || saved || !preferred}
                >
                    {saved ? '✓ ' : ''}
                    {idle && !saved && idleLabel ? idleLabel : downloadLabel(progress, inLibrary)}
                </button>
                {active && (
                    <button type="button" className="btn secondary" onClick={onCancel}>
                        Cancel
                    </button>
                )}
            </div>
            {!active && !saved && preferred && <p className="notice">{preferred.label}</p>}
            {!active && !saved && other && (
                <button type="button" className="link download-other" onClick={() => onDownload(other.size)}>
                    Download as: {other.label}
                </button>
            )}
            {active && (
                <progress
                    className="download-progress"
                    max={100}
                    value={pct ?? undefined}
                    aria-label="Download progress"
                />
            )}
            {message && (failed || progress?.state !== 'failed') && (
                <p className={failed ? 'notice error' : 'notice'} role={failed ? 'alert' : 'status'}>
                    {message}
                </p>
            )}
            {fetchedId && onKeepCopy ? (
                <button type="button" className="btn secondary" onClick={() => onKeepCopy(fetchedId)}>
                    Save the screen-size copy instead
                </button>
            ) : null}
        </div>
    );
}
