import {useEffect, useState} from 'react';
import {GetUpdateState} from '../../wailsjs/go/main/App';
import {EventsOn} from '../../wailsjs/runtime/runtime';
import {main} from '../../wailsjs/go/models';

// useUpdate returns what is known about a newer version, kept current.
export function useUpdate(): main.UpdateState | null {
    const [state, setState] = useState<main.UpdateState | null>(null);
    useEffect(() => {
        const read = () => {
            GetUpdateState()
                .then(setState)
                .catch(() => {}); // only a convenience
        };
        read();
        return EventsOn('update:state', read);
    }, []);
    return state;
}
