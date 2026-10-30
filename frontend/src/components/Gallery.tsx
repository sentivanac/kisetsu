import {images} from '../../wailsjs/go/models';
import {imageKey} from '../lib/images';

interface Props {
    items: images.Image[];
    onSelect: (index: number) => void;
}

// Gallery shows search results as a responsive grid of thumbnails.
// Thumbnails load lazily, so only visible rows are downloaded.
export function Gallery({items, onSelect}: Props) {
    return (
        <ul className="gallery">
            {items.map((img, index) => (
                <li key={imageKey(img)}>
                    <button
                        type="button"
                        className="gallery-item"
                        onClick={() => onSelect(index)}
                        aria-label={`Open preview, ${img.width} by ${img.height}`}
                    >
                        <img
                            src={img.thumbnailUrl}
                            alt=""
                            loading="lazy"
                            decoding="async"
                            width={300}
                            height={200}
                        />
                        <span className="gallery-badge">
                            {img.width} × {img.height}
                        </span>
                    </button>
                </li>
            ))}
        </ul>
    );
}
