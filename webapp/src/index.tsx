import React from 'react';

import {ScheduleAPI} from './api';
import {Settings} from './settings';

const pluginID = 'org.mattermost.scheduled-dnd';

type Store = {getState(): {entities?: {general?: {config?: {SiteURL?: string}}}}};
type Registry = {
    registerUserSettings(settings: {
        id: string;
        uiName: string;
        sections: {title: string; component: React.ComponentType}[];
    }): void;
};

declare global {
    interface Window {
        registerPlugin(id: string, plugin: {initialize(registry: Registry, store: Store): void; uninitialize(): void}): void;
    }
}

class ScheduledDND {
    initialize(registry: Registry, store: Store) {
        // Preserve URL subpaths while keeping requests on the current origin.
        const siteURL = store.getState().entities?.general?.config?.SiteURL;
        const subpath = siteURL ? new URL(siteURL, window.location.origin).pathname.replace(/\/$/, '') : '';
        const api = new ScheduleAPI(`${subpath}/plugins/${pluginID}`);
        registry.registerUserSettings({
            id: pluginID,
            uiName: 'Scheduled DND',
            sections: [{title: 'Schedule', component: () => <Settings api={api}/>}],
        });
    }

    uninitialize() {
        // Mattermost removes plugin registrations and unmounts their components.
    }
}

window.registerPlugin(pluginID, new ScheduledDND());
