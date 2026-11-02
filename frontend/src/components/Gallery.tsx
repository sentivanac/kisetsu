import {images} from '../../wailsjs/go/models';
import {imageKey, orientationOf} from '../lib/images';

interface Props {
    items: images.Image[];
    sourceName: (providerId: string) => string;
    // Keys of images already in the library.
    inLibrary?: (key: string) => boolean;
    // Download progress from 0 to 100, null when unknown, undefined when
    // the image is not being downloaded.
    downloadPercent?: (key: string) => number | null | undefined;
    onSelect: (index: number) => void;
}

// Gallery shows search results as a responsive grid of thumbnails.
// Thumbnails load lazily, so only visible rows are downloaded.
export function Gallery({items, sourceName, inLibrary, downloadPercent, onSelect}: Props) {
    return (
        <ul className="gallery">
            {items.map((img, index) => {
                const key = imageKey(img);
                const saved = inLibrary?.(key) ?? false;
                const pct = downloadPercent?.(key);
                // Thumbnails are cropped to 3:2, so the shape is marked.
                const shape = orientationOf(img.width, img.height);
                return (
                <li key={key}>
                    <button
                        type="button"
                        className="gallery-item"
                        onClick={() => onSelect(index)}
                        aria-label={`Open preview, ${shape ? shape + ', ' : ''}${img.width} by ${img.height}, from ${sourceName(img.provider)}${saved ? ', in your library' : ''}`}
                    >
                        <img
                            src={img.thumbnailUrl}
                            alt=""
                            loading="lazy"
                            decoding="async"
                            width={300}
                            height={200}
                        />
                        {shape === 'portrait' && (
                            <span className="gallery-badge gallery-shape" title="Portrait">
                                ▯<span className="sr-only">Portrait</span>
                            </span>
                        )}
                        {shape === 'square' && (
                            <span className="gallery-badge gallery-shape" title="Square">
                                □<span className="sr-only">Square</span>
                            </span>
                        )}
                        <span className="gallery-badge gallery-source">{sourceName(img.provider)}</span>
                        {saved && (
                            <span className="gallery-badge gallery-saved" title="In your library">
                                ✓<span className="sr-only">In your library</span>
                            </span>
                        )}
                        <span className="gallery-badge">
                            {img.width} × {img.height}
                        </span>
                        {pct !== undefined && (
                            <progress className="gallery-progress" max={100} value={pct ?? undefined}/>
                        )}
                    </button>
                </li>
                );
            })}
        </ul>
    );
}
