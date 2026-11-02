// Error handling for calls into the Go backend.
//
// Go methods reject with apperr.PublicError ({code, message, retryAfterSeconds?}).
// Wails itself rejects with a plain string for some failures (for example
// when arguments cannot be parsed), so both shapes are handled.

export type ErrorCode =
    | 'internal'
    | 'unavailable'
    | 'invalid_input'
    | 'rate_limited'
    | 'provider_unavailable'
    | 'canceled';

export interface BackendError {
    code: ErrorCode | string;
    message: string;
    retryAfterSeconds?: number;
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

// describeError returns the text to show for an error.
export function describeError(err: BackendError): string {
    const seconds = err.retryAfterSeconds ?? 0;
    if (seconds > 0) {
        return `${err.message} Try again in ${seconds} s.`;
    }
    return err.message;
}

// isCanceled reports whether an error only means that the operation was
// stopped on purpose (for example replaced by a newer search). Such errors
// are not shown to the user.
export function isCanceled(err: BackendError): boolean {
    return err.code === 'canceled';
}
