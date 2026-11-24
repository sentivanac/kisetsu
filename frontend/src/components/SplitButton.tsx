import {KeyboardEvent, useEffect, useId, useRef, useState} from 'react';

interface Props {
    label: string;
    disabled: boolean;
    onClick: () => void;
    // Names the arrow button for screen readers.
    menuLabel: string;
    // With fewer than two, there is no menu.
    options: {id: string; label: string}[];
    onSelect: (id: string) => void;
}

// SplitButton is a button with an arrow that opens a menu of other ways to
// do the same (the WAI-ARIA menu button pattern).
export function SplitButton({label, disabled, onClick, menuLabel, options, onSelect}: Props) {
    const [open, setOpen] = useState(false);
    const root = useRef<HTMLDivElement>(null);
    const arrow = useRef<HTMLButtonElement>(null);
    const menu = useRef<HTMLDivElement>(null);
    const menuId = useId();
    const hasMenu = options.length > 1 && !disabled;
    const shown = open && hasMenu;

    const items = () => [...(menu.current?.querySelectorAll<HTMLButtonElement>('[role=menuitem]') ?? [])];

    useEffect(() => {
        if (!shown) return;
        items()[0]?.focus();
        const closeOutside = (event: PointerEvent) => {
            if (!root.current?.contains(event.target as Node)) setOpen(false);
        };
        document.addEventListener('pointerdown', closeOutside);
        return () => document.removeEventListener('pointerdown', closeOutside);
    }, [shown]);

    function close() {
        setOpen(false);
        arrow.current?.focus();
    }

    function onMenuKey(event: KeyboardEvent) {
        if (event.key === 'Tab') {
            setOpen(false);
            return;
        }
        // Keys in the menu are not for the dialog around it: Escape would
        // close the preview and the arrows would change the image.
        event.stopPropagation();
        const list = items();
        const index = list.indexOf(document.activeElement as HTMLButtonElement);
        const focus = (i: number) => list[(i + list.length) % list.length]?.focus();
        switch (event.key) {
            case 'Escape':
                close();
                break;
            case 'ArrowDown':
                focus(index + 1);
                break;
            case 'ArrowUp':
                focus(index - 1);
                break;
            case 'Home':
                focus(0);
                break;
            case 'End':
                focus(-1);
                break;
            default:
                return;
        }
        event.preventDefault();
    }

    return (
        <div className={hasMenu ? 'split split-menu' : 'split'} ref={root}>
            <button type="button" className="btn" onClick={onClick} disabled={disabled}>
                {label}
            </button>
            {hasMenu && (
                <button
                    ref={arrow}
                    type="button"
                    className="btn split-arrow"
                    aria-label={menuLabel}
                    aria-haspopup="menu"
                    aria-expanded={shown}
                    aria-controls={shown ? menuId : undefined}
                    onClick={() => setOpen(!open)}
                    onKeyDown={(event) => {
                        if (event.key !== 'ArrowDown') return;
                        event.preventDefault();
                        setOpen(true);
                    }}
                >
                    <span aria-hidden="true">▾</span>
                </button>
            )}
            {shown && (
                <div ref={menu} id={menuId} className="split-list" role="menu" aria-label={menuLabel} onKeyDown={onMenuKey}>
                    {options.map((o) => (
                        <button
                            key={o.id}
                            type="button"
                            role="menuitem"
                            tabIndex={-1}
                            onClick={() => {
                                close();
                                onSelect(o.id);
                            }}
                        >
                            {o.label}
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
}
