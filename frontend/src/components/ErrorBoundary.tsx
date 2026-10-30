import {Component, ErrorInfo, ReactNode} from 'react';

interface Props {
    children: ReactNode;
}

interface State {
    failed: boolean;
}

// Keeps a rendering error from leaving the window blank.
// React only supports error boundaries as class components.
export class ErrorBoundary extends Component<Props, State> {
    state: State = {failed: false};

    static getDerivedStateFromError(): State {
        return {failed: true};
    }

    componentDidCatch(error: Error, info: ErrorInfo) {
        console.error('UI error', error, info.componentStack);
    }

    render() {
        if (this.state.failed) {
            return (
                <main className="app" role="alert">
                    <h1 className="title">Something went wrong</h1>
                    <p className="tagline">The interface hit an unexpected error.</p>
                    <button type="button" className="btn" onClick={() => window.location.reload()}>
                        Reload
                    </button>
                </main>
            );
        }
        return this.props.children;
    }
}
