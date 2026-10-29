import assert from 'node:assert/strict';
import test from 'node:test';

import {ScheduleAPI} from '../src/api.ts';
import type {Fetcher} from '../src/api.ts';
import {newRule} from '../src/model.ts';

test('API uses same-origin credentials, subpath and no supplied user identity', async () => {
    const calls: {url: string; options: RequestInit}[] = [];
    const fetcher: Fetcher = async (url, options) => {
        calls.push({url: String(url), options: options!});
        return new Response(JSON.stringify({revision: 3}), {status: 200});
    };
    const api = new ScheduleAPI('/mattermost/plugins/org.mattermost.scheduled-dnd', fetcher);
    const config = {enabled: true, timezone: 'Europe/Warsaw', schedule: [newRule()]};
    await api.load();
    await api.save(config, 2);
    await api.reset(3);
    await api.skip();
    assert.equal(calls.length, 4);
    for (const call of calls) {
        assert.ok(call.url.startsWith('/mattermost/plugins/org.mattermost.scheduled-dnd/api/v1/'));
        assert.equal(call.options.credentials, 'same-origin');
        assert.equal(new Headers(call.options.headers).get('X-Requested-With'), 'XMLHttpRequest');
        assert.equal(new Headers(call.options.headers).get('Mattermost-User-ID'), null);
    }
    assert.deepEqual(JSON.parse(calls[1].options.body as string), {config, revision: 2});
    assert.equal(new Headers(calls[2].options.headers).get('If-Match'), '"3"');
    assert.equal(calls[2].options.method, 'DELETE');
    assert.equal(calls[3].options.method, 'POST');
    assert.equal(calls[3].options.body, undefined);
});

test('API presents server validation and concurrent edit errors', async () => {
    for (const [status, message] of [[400, 'invalid timezone'], [409, 'configuration changed; reload before saving'], [401, 'authentication required']] as const) {
        const fetcher: Fetcher = async () => new Response(JSON.stringify({error: message}), {status});
        await assert.rejects(new ScheduleAPI('/plugins/id', fetcher).load(), {message});
    }
});

test('unstructured server failures and network failures are surfaced', async () => {
    const server: Fetcher = async () => new Response('{}', {status: 503});
    await assert.rejects(new ScheduleAPI('/plugins/id', server).load(), /503/);
    const offline: Fetcher = async () => { throw new Error('offline'); };
    await assert.rejects(new ScheduleAPI('/plugins/id', offline).load(), /offline/);
});
