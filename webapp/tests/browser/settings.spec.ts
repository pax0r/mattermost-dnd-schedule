import {test, expect} from '@playwright/test';

const baseURL = process.env.BROWSER_BASE_URL!;
const configURL = `${baseURL}/mattermost/plugins/org.mattermost.scheduled-dnd/api/v1/config`;
const headers = {'X-Requested-With': 'XMLHttpRequest'};

test.beforeEach(async ({page, request, context}) => {
    await request.post(`${baseURL}/fixture/reset`);
    await context.addCookies([{name: 'session', value: 'fixture-session', url: baseURL}]);
    await page.goto(`${baseURL}/mattermost/`);
    await page.getByRole('button', {name: 'Settings', exact: true}).click();
    await page.getByRole('button', {name: 'Scheduled DND', exact: true}).click();
});

test('compiled plugin loads in user settings and saves through native fetch', async ({page}) => {
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await expect(page.getByRole('switch')).toBeChecked();
    await expect(page.getByLabel('Timezone', {exact: true})).toHaveValue('Europe/Warsaw');
    await page.getByLabel('Start', {exact: true}).fill('18:30');
    await page.getByRole('button', {name: 'Save schedule', exact: true}).click();
    await expect(page.getByRole('status').filter({hasText: 'Schedule saved.'})).toBeVisible();
    await page.reload();
    await page.getByRole('button', {name: 'Settings', exact: true}).click();
    await page.getByRole('button', {name: 'Scheduled DND', exact: true}).click();
    await expect(page.getByLabel('Start', {exact: true})).toHaveValue('18:30');
    expect(errors).toEqual([]);
});

test('validates drafts and supports multiple intervals', async ({page}) => {
    await page.getByLabel('Timezone', {exact: true}).fill('Not/A_Zone');
    await expect(page.getByRole('alert')).toContainText('Unknown timezone');
    await expect(page.getByRole('button', {name: 'Save schedule', exact: true})).toBeDisabled();
    await page.getByLabel('Timezone', {exact: true}).fill('Europe/Warsaw');
    await page.getByRole('button', {name: '+ Add interval', exact: true}).click();
    const interval = page.getByRole('group', {name: 'Interval 2', exact: true});
    await expect(interval).toBeVisible();
    await interval.getByLabel('Start', {exact: true}).fill('10:00');
    await interval.getByLabel('End', {exact: true}).fill('12:00');
    await page.getByRole('button', {name: 'Save schedule', exact: true}).click();
    await expect(page.getByRole('status').filter({hasText: 'Schedule saved.'})).toBeVisible();
});

test('revision conflicts preserve draft until explicit reload', async ({page, context}) => {
    await expect(page.getByRole('switch')).toBeChecked();
    const saved = await (await context.request.get(configURL)).json();
    const config = {...saved.config, timezone: 'UTC'};
    expect((await context.request.put(configURL, {headers, data: {config, revision: saved.revision}})).ok()).toBeTruthy();
    await page.getByLabel('Start', {exact: true}).fill('19:00');
    await page.getByRole('button', {name: 'Save schedule', exact: true}).click();
    await expect(page.getByRole('alert')).toContainText('configuration changed');
    await expect(page.getByLabel('Start', {exact: true})).toHaveValue('19:00');
    await page.getByRole('button', {name: 'Reload saved configuration'}).click();
    await expect(page.getByLabel('Timezone', {exact: true})).toHaveValue('UTC');
    await expect(page.getByLabel('Start', {exact: true})).toHaveValue('17:00');
});

test('skip and reset use the backend and retain safe status behavior', async ({page, context}) => {
    await page.getByRole('button', {name: 'Skip current interval', exact: true}).click();
    await expect(page.getByLabel('Current automation status')).toContainText('paused for this interval');
    await expect(page.getByLabel('Current automation status')).toContainText('Mattermost status: offline');
    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', {name: 'Reset configuration', exact: true}).click();
    await expect(page.getByRole('switch')).not.toBeChecked();
    await expect(page.getByText('No intervals configured.', {exact: true})).toBeVisible();
    const saved = await (await context.request.get(configURL)).json();
    expect(saved.config.enabled).toBe(false);
    expect(saved.config.schedule).toEqual([]);
    expect(saved.runtime.suppress_until).toBeGreaterThan(0);
});

test('authentication errors are visible and configuration can be reloaded', async ({page, context}) => {
    await expect(page.getByRole('switch')).toBeChecked();
    await context.clearCookies();
    await page.getByLabel('Start', {exact: true}).fill('20:00');
    await page.getByRole('button', {name: 'Save schedule', exact: true}).click();
    await expect(page.getByRole('alert')).toContainText('authentication required');
    await context.addCookies([{name: 'session', value: 'fixture-session', url: baseURL}]);
    await page.getByRole('button', {name: 'Reload saved configuration'}).click();
    await expect(page.getByLabel('Start', {exact: true})).toHaveValue('17:00');
});

test('server failures show a recoverable error', async ({page, request}) => {
    await expect(page.getByRole('switch')).toBeChecked();
    await request.post(`${baseURL}/fixture/unavailable`);
    await page.getByLabel('Start', {exact: true}).fill('20:00');
    await page.getByRole('button', {name: 'Save schedule', exact: true}).click();
    await expect(page.getByRole('alert')).toContainText('request failed');
    await expect(page.getByLabel('Start', {exact: true})).toHaveValue('20:00');
    await request.post(`${baseURL}/fixture/reset`);
    await page.getByRole('button', {name: 'Reload saved configuration'}).click();
    await expect(page.getByLabel('Start', {exact: true})).toHaveValue('17:00');
});
