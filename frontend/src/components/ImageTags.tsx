import {useEffect, useRef, useState} from 'react';
import {ImageTags as LoadImageTags} from '../../wailsjs/go/main/App';
import {images} from '../../wailsjs/go/models';
import {isTagHidden} from '../lib/tags';

interface Props {
    image: images.Image;
    hidden: string[];
    onToggle: (tag: string, hide: boolean) => void;
}

// Tags are loaded after the image has been shown for a moment, so paging
// quickly through previews does not spend the source's API limit.
const loadDelay = 400;

// ImageTags lists the tags of a previewed image. Clicking a tag hides
// images with it from future searches, or shows them again.
export function ImageTags({image, hidden, onToggle}: Props) {
    const key = `${image.provider}:${image.providerId}`;
    // Loaded tags by image, kept while the preview is open.
    const cache = useRef(new Map<string, string[]>());
    const [tags, setTags] = useState<{key: string; list: string[]} | null>(null);
    const [failed, setFailed] = useState<string | null>(null);

    useEffect(() => {
        const cached = cache.current.get(key);
        if (cached) {
            setTags({key, list: cached});
            return;
        }
        let current = true;
        const timer = window.setTimeout(() => {
            LoadImageTags(image.provider, image.providerId)
                .then((list) => {
                    cache.current.set(key, list);
                    if (current) setTags({key, list});
                })
                .catch(() => {
                    if (current) setFailed(key);
                });
        }, loadDelay);
        return () => {
            current = false;
            window.clearTimeout(timer);
        };
    }, [key]);

    const list = tags?.key === key ? tags.list : null;
    return (
        <section className="image-tags" aria-label="Tags">
            <h3>Tags</h3>
            {list === null && failed !== key && <p className="muted">Loading tags…</p>}
            {list === null && failed === key && <p className="muted">Tags could not be loaded.</p>}
            {list !== null && list.length === 0 && <p className="muted">This image has no tags.</p>}
            {list !== null && list.length > 0 && (
                <>
                    <div className="chips">
                        {list.map((tag) => {
                            const isHidden = isTagHidden(hidden, tag);
                            return (
                                <button
                                    key={tag}
                                    type="button"
                                    className={isHidden ? 'chip tag-hidden' : 'chip'}
                                    aria-pressed={isHidden}
                                    title={
                                        isHidden ? `Show images tagged “${tag}” again` : `Hide images tagged “${tag}”`
                                    }
                                    onClick={() => onToggle(tag, !isHidden)}
                                >
                                    {tag}
                                </button>
                            );
                        })}
                    </div>
                    <p className="muted small">Click a tag to hide images with it from new searches.</p>
                </>
            )}
        </section>
    );
}
