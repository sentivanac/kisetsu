// Error handling for calls into the Go backend.
//
// Go methods reject with apperr.Public ({code, message}). Wails itself
// rejects with a plain string for some failures (for example when
// arguments cannot be parsed), so both shapes are handled.

export type ErrorCode = 'internal' | 'unavailable';

export interface BackendError {
    code: ErrorCode | string;
    message: string;
}

const fallbackMessage = 'Something went wrong. Details were written to the log file.';

function isBackendError(value: unknown): value is BackendError {
    return (
        typeof value === 'object' &&
        value !== null &&
        typeof (value as BackendError).code === 'string' &&
        typeof (value as BackendError).message === 'string'
    );
}

export function toBackendError(err: unknown): BackendError {
    if (isBackendError(err)) {
        return err;
    }
    if (typeof err === 'string' && err.trim() !== '') {
        return {code: 'internal', message: err};
    }
    return {code: 'internal', message: fallbackMessage};
}
