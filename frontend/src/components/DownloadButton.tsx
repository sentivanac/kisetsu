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
import {SplitButton} from './SplitButton';

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
    const [preferred] = choices;
    const idle = !progress || progress.state === 'canceled';
    const fetchedId = failed ? progress?.fetchedId : undefined;

    return (
        <div className="download">
            <div className="download-row">
                <SplitButton
                    label={
                        (saved ? '✓ ' : '') +
                        (idle && !saved && idleLabel ? idleLabel : downloadLabel(progress, inLibrary))
                    }
                    disabled={active || saved || !preferred}
                    onClick={() => onDownload(preferred.size)}
                    menuLabel="Choose the download size"
                    options={choices.map((c) => ({id: c.size, label: c.menuLabel}))}
                    onSelect={(size) => onDownload(size as DownloadSize)}
                />
                {active && (
                    <button type="button" className="btn secondary" onClick={onCancel}>
                        Cancel
                    </button>
                )}
            </div>
            {!active && !saved && preferred && <p className="notice">{preferred.label}</p>}
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
                    Save the screen copy instead
                </button>
            ) : null}
        </div>
    );
}
