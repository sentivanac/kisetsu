import {useEffect, useRef, useState} from 'react';
import {GetAbout, GetThirdPartyLicenses, SetUpdateCheck} from '../../wailsjs/go/main/App';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {main} from '../../wailsjs/go/models';
import {useUpdate} from '../lib/useUpdate';
import icon from '../assets/icon.png';

interface Props {
    onClose: () => void;
}

// AboutDialog shows the version, the author, the license and the credits:
// where images come from, and the third-party code built into Kisetsu.
export function AboutDialog({onClose}: Props) {
    const dialog = useRef<HTMLDialogElement>(null);
    const [about, setAbout] = useState<main.About | null>(null);
    // The third-party licenses, read when asked for: about 160 KB.
    const [licenses, setLicenses] = useState<string | null>(null);
    const [licensesError, setLicensesError] = useState(false);
    const update = useUpdate();
    const [updateError, setUpdateError] = useState(false);

    // The result arrives through useUpdate.
    function setUpdateCheck(on: boolean) {
        setUpdateError(false);
        SetUpdateCheck(on).catch(() => setUpdateError(true));
    }

    function showLicenses() {
        setLicensesError(false);
        GetThirdPartyLicenses()
            .then(setLicenses)
            .catch(() => setLicensesError(true));
    }

    useEffect(() => {
        const el = dialog.current;
        if (el && !el.open) el.showModal();
        GetAbout()
            .then(setAbout)
            .catch(() => {}); // the dialog still shows the credits
    }, []);

    const link = (url: string, label: string) => (
        <button type="button" className="link" onClick={() => BrowserOpenURL(url)}>
            {label}
        </button>
    );

    return (
        <dialog ref={dialog} className="about" onClose={onClose} aria-labelledby="about-title">
            {/* Only the body scrolls, so Close stays visible in a low window. */}
            <div className="about-body">
                <div className="about-columns">
                    <section className="about-app">
                        <img className="about-icon" src={icon} alt="" />
                        <h2 id="about-title">
                            Kisetsu <span lang="ja">季節</span>
                        </h2>
                        {about && <p className="muted">Version {about.version}</p>}
                        <p>Discover wallpapers, keep them in a library, and change them automatically.</p>
                        {about && (
                            <>
                                <p>
                                    © 2026 {link(about.authorUrl, about.author)}
                                    <br />
                                    {link(about.licenseUrl, `${about.license} license`)}
                                </p>
                                <p>
                                    {link(about.projectUrl, 'Website')} · {link(about.issuesUrl, 'Report a problem')}
                                </p>
                            </>
                        )}
                        {update && (
                            <div className="about-update">
                                {update.available && (
                                    <p>{link(update.url, `Version ${update.available} is available`)}</p>
                                )}
                                {update.available && update.viaSystem && (
                                    <p className="muted small">
                                        It comes with your system's updates: Update Manager, Software Updater or{' '}
                                        <code>sudo apt upgrade</code>.
                                    </p>
                                )}
                                <label>
                                    <input
                                        type="checkbox"
                                        checked={update.enabled}
                                        onChange={(e) => setUpdateCheck(e.target.checked)}
                                    />
                                    Check for new versions
                                </label>
                                <p className="muted small">
                                    Asks GitHub once a day. Nothing is downloaded or installed.
                                </p>
                                {updateError && <p className="error small">The setting could not be saved.</p>}
                            </div>
                        )}
                    </section>

                    <section className="about-credits" aria-labelledby="about-credits">
                        <h3 id="about-credits">Credits</h3>
                        <ul>
                            <li>
                                Images from Wallhaven, Wikimedia Commons and Openverse belong to their authors;
                                each image shows its author and license. Kisetsu uses Openverse but is not
                                endorsed by it.
                            </li>
                            <li>Text on the wallpaper uses the Go fonts (BSD license).</li>
                            <li>Kisetsu is built with Wails, React and other open-source software.</li>
                        </ul>
                        {about?.hasThirdPartyLicenses && licenses === null && (
                            <button type="button" className="btn secondary" onClick={showLicenses}>
                                Third-party licenses
                            </button>
                        )}
                        {licensesError && <p className="error small">The list of licenses could not be read.</p>}
                    </section>
                </div>

                {licenses !== null && (
                    <pre className="about-licenses" tabIndex={0} aria-label="Third-party licenses">
                        {licenses}
                    </pre>
                )}
            </div>

            <form method="dialog" className="about-footer">
                <span className="muted small">{about && `${about.platform} · ${about.goVersion}`}</span>
                <button type="submit" className="btn" autoFocus>
                    Close
                </button>
            </form>
        </dialog>
    );
}
