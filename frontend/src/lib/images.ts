import type {images} from '../../wailsjs/go/models';

type Image = images.Image;

// imageKey identifies an image across providers.
export function imageKey(img: Pick<Image, 'provider' | 'providerId'>): string {
    return `${img.provider}/${img.providerId}`;
}

// appendUnique adds the images of a new page, skipping ones already shown.
// Provider pages can overlap when results change between requests.
export function appendUnique<T extends Pick<Image, 'provider' | 'providerId'>>(existing: T[], incoming: T[]): T[] {
    const seen = new Set(existing.map(imageKey));
    const added = incoming.filter((img) => {
        const key = imageKey(img);
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
    });
    return added.length === 0 ? existing : [...existing, ...added];
}

// formatFileSize returns a short size such as "1.2 MB", or "" if unknown.
export function formatFileSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit++;
    }
    const digits = unit === 0 || value >= 10 ? 0 : 1;
    return `${value.toFixed(digits)} ${units[unit]}`;
}

// formatType turns a MIME type such as "image/jpeg" into "JPEG".
export function formatType(mime: string): string {
    const subtype = mime.split('/')[1] ?? '';
    return subtype.replace(/^x-/, '').toUpperCase();
}

// licenseLabel is the short license text shown with an image.
export function licenseLabel(license: Pick<images.License, 'status' | 'name'>): string {
    return license.status === 'known' && license.name ? license.name : 'License unknown';
}
