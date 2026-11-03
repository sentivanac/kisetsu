// Error handling for calls into the Go backend.
//
// Go methods fail with apperr.PublicError ({code, message, retryAfterSeconds?}),
// which the ErrorFormatter (errorformat.go) sends as JSON text. The Wails v2
// runtime rejects the promise with new Error(thatText), so the JSON arrives
// as the Error's message. Wails also rejects with plain text for its own
// failures (for example when arguments cannot be parsed).

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

// parseBackendError reads an error sent as JSON text, or returns null.
function parseBackendError(text: string): BackendError | null {
    try {
        const value: unknown = JSON.parse(text);
        return isBackendError(value) ? value : null;
    } catch {
        return null;
    }
}

export function toBackendError(err: unknown): BackendError {
    if (isBackendError(err)) {
        return err;
    }
    if (err instanceof Error) {
        // Other Error messages (for example Wails' "Call to … timed out")
        // are technical, so they get the generic text.
        return parseBackendError(err.message) ?? {code: 'internal', message: fallbackMessage};
    }
    if (typeof err === 'string' && err.trim() !== '') {
        return parseBackendError(err) ?? {code: 'internal', message: err};
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
