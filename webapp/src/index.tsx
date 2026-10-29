import React, {useEffect, useState} from 'react';

import {ScheduleAPI} from './api';
import {Settings} from './settings';

const pluginID = 'org.mattermost.scheduled-dnd';
const openEvent = `${pluginID}:open`;
const closeEvent = `${pluginID}:close`;

type Store = {getState(): {entities?: {general?: {config?: {SiteURL?: string}}}}};
type Registry = {
    registerRootComponent(component: React.ComponentType): string;
    registerMainMenuAction(text: string, action: () => void): string;
};

declare global {
    interface Window {
        registerPlugin(id: string, plugin: {initialize(registry: Registry, store: Store): void; uninitialize(): void}): void;
    }
}

class ScheduledDND {
    initialize(registry: Registry, store: Store) {
        // SiteURL preserves Mattermost deployments under a URL subpath. Keep
        // requests on the current origin rather than trusting an arbitrary URL.
        const siteURL = store.getState().entities?.general?.config?.SiteURL;
        const subpath = siteURL ? new URL(siteURL, window.location.origin).pathname.replace(/\/$/, '') : '';
        const api = new ScheduleAPI(`${subpath}/plugins/${pluginID}`);
        function Root() {
            const [open, setOpen] = useState(false);
            useEffect(() => {
                const show = () => setOpen(true);
                const hide = () => setOpen(false);
                window.addEventListener(openEvent, show);
                window.addEventListener(closeEvent, hide);
                return () => {
                    window.removeEventListener(openEvent, show);
                    window.removeEventListener(closeEvent, hide);
                };
            }, []);
            return open ? <Settings api={api} onClose={() => setOpen(false)}/> : null;
        }
        registry.registerRootComponent(Root);
        registry.registerMainMenuAction('Scheduled DND', () => window.dispatchEvent(new Event(openEvent)));
    }

    uninitialize() {
        window.dispatchEvent(new Event(closeEvent));
    }
}

window.registerPlugin(pluginID, new ScheduledDND());
