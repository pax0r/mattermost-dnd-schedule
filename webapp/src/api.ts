import type {Config, View} from './model';

export type Fetcher = typeof fetch;
export class ScheduleAPI {
    private readonly base: string;
    private readonly fetcher: Fetcher;

    constructor(base: string, fetcher: Fetcher = fetch) {
        this.base = base;
        this.fetcher = fetcher;
    }

    private async request(path: string, options: RequestInit = {}): Promise<View> {
        const response = await this.fetcher(`${this.base}/api/v1/${path}`, {
            ...options,
            credentials: 'same-origin',
            headers: {'X-Requested-With': 'XMLHttpRequest', ...options.headers},
        });
        const body = await response.json();
        if (!response.ok) {
            throw new Error(typeof body.error === 'string' ? body.error : `Request failed (${response.status})`);
        }
        return body as View;
    }

    load(): Promise<View> {
        return this.request('config');
    }

    save(config: Config, revision: number): Promise<View> {
        return this.request('config', {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({config, revision}),
        });
    }

    reset(revision: number): Promise<View> {
        return this.request('config', {method: 'DELETE', headers: {'If-Match': `"${revision}"`}});
    }

    skip(): Promise<View> {
        return this.request('skip', {method: 'POST'});
    }
}
