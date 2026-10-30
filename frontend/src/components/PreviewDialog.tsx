import {KeyboardEvent, ReactNode, useEffect, useRef, useState} from 'react';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {images} from '../../wailsjs/go/models';
import {formatFileSize, formatType, licenseLabel} from '../lib/images';

interface Props {
    image: images.Image;
    providerName: string;
    // Rendered under the details, e.g. a download button.
    actions?: ReactNode;
    onClose: () => void;
    onPrevious?: () => void;
    onNext?: () => void;
}

// PreviewDialog shows one image large, with its details and attribution.
// The uncropped thumbnail appears immediately; the full image replaces it
// once downloaded. The full file is only fetched when a preview is opened.
export function PreviewDialog({image, providerName, actions, onClose, onPrevious, onNext}: Props) {
    const dialog = useRef<HTMLDialogElement>(null);
    // Status is derived from which URL finished loading, so switching images
    // cannot leave a stale "loaded" or "loading" state behind.
    const [result, setResult] = useState<{url: string; ok: boolean} | null>(null);
    const status = result?.url !== image.downloadUrl ? 'loading' : result.ok ? 'loaded' : 'failed';
    // The blurred placeholder is removed once the full image has faded in,
    // so it can never show through again.
    const [fadedUrl, setFadedUrl] = useState<string | null>(null);
    const showPlaceholder = fadedUrl !== image.downloadUrl;

    useEffect(() => {
        const el = dialog.current;
        if (el && !el.open) el.showModal();
    }, []);

    useEffect(() => {
        if (status !== 'loaded') return;
        const url = image.downloadUrl;
        // Slightly longer than the CSS fade (0.3 s).
        const timer = window.setTimeout(() => setFadedUrl(url), 350);
        return () => window.clearTimeout(timer);
    }, [status, image.downloadUrl]);

    function onKeyDown(event: KeyboardEvent) {
        if (event.key === 'ArrowLeft' && onPrevious) {
            event.preventDefault();
            onPrevious();
        } else if (event.key === 'ArrowRight' && onNext) {
            event.preventDefault();
            onNext();
        }
    }

    const size = formatFileSize(image.fileSize);
    const type = formatType(image.mimeType);

    return (
        <dialog ref={dialog} className="preview" onClose={onClose} onKeyDown={onKeyDown} aria-label="Image preview">
            <div className="preview-image">
                {showPlaceholder && (
                    <img
                        key={image.previewUrl}
                        className="preview-placeholder"
                        src={image.previewUrl || image.thumbnailUrl}
                        alt=""
                        aria-hidden="true"
                    />
                )}
                <img
                    key={image.downloadUrl}
                    className={status === 'loaded' ? 'preview-full loaded' : 'preview-full'}
                    src={image.downloadUrl}
                    alt={`Image from ${providerName}, ${image.width} by ${image.height}`}
                    onLoad={() => setResult({url: image.downloadUrl, ok: true})}
                    onError={() => setResult({url: image.downloadUrl, ok: false})}
                />
                {status === 'loading' && <p className="preview-status">Loading full image…</p>}
                {status === 'failed' && <p className="preview-status error">The full image could not be loaded.</p>}
            </div>

            <aside className="preview-details">
                <dl className="info">
                    <dt>Size</dt>
                    <dd>
                        {image.width} × {image.height}
                    </dd>
                    {(type || size) && (
                        <>
                            <dt>File</dt>
                            <dd>{[type, size].filter(Boolean).join(' · ')}</dd>
                        </>
                    )}
                    <dt>Source</dt>
                    <dd>{providerName}</dd>
                    <dt>Author</dt>
                    <dd>
                        {image.author.name ? (
                            image.author.url ? (
                                <button type="button" className="link" onClick={() => BrowserOpenURL(image.author.url)}>
                                    {image.author.name}
                                </button>
                            ) : (
                                image.author.name
                            )
                        ) : (
                            'Unknown'
                        )}
                    </dd>
                    <dt>License</dt>
                    <dd>
                        {image.license.url ? (
                            <button type="button" className="link" onClick={() => BrowserOpenURL(image.license.url)}>
                                {licenseLabel(image.license)}
                            </button>
                        ) : (
                            licenseLabel(image.license)
                        )}
                    </dd>
                </dl>
                {image.license.notice && <p className="notice">{image.license.notice}</p>}

                {actions}

                <div className="preview-actions">
                    <button type="button" className="btn secondary" onClick={() => BrowserOpenURL(image.pageUrl)}>
                        Open on {providerName}
                    </button>
                    {image.sourceUrl && (
                        <button type="button" className="btn secondary" onClick={() => BrowserOpenURL(image.sourceUrl)}>
                            Original source
                        </button>
                    )}
                </div>

                <div className="preview-nav">
                    <button type="button" className="btn secondary" onClick={onPrevious} disabled={!onPrevious}>
                        ← Previous
                    </button>
                    <button type="button" className="btn secondary" onClick={onNext} disabled={!onNext}>
                        Next →
                    </button>
                    <button type="button" className="btn secondary" onClick={() => dialog.current?.close()}>
                        Close
                    </button>
                </div>
            </aside>
        </dialog>
    );
}
