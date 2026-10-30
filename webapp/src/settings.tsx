import React, {useEffect, useRef, useState} from 'react';

import type {ScheduleAPI} from './api';
import {automationLabels, days, newRule, summary, validate} from './model';
import type {Config, Rule, View} from './model';
import {styles} from './styles';

type Props = {api: ScheduleAPI};

function formatDate(value: string, timezone: string): string {
    return new Intl.DateTimeFormat(undefined, {timeZone: timezone, dateStyle: 'medium', timeStyle: 'short'}).format(new Date(value));
}

function timezones(): string[] {
    const intl = Intl as typeof Intl & {supportedValuesOf?: (key: string) => string[]};
    return ['UTC', ...(intl.supportedValuesOf?.('timeZone') ?? ['Europe/Warsaw', 'Europe/London', 'America/New_York', 'Asia/Tokyo'])];
}

export function Settings({api}: Props) {
    const [view, setView] = useState<View | null>(null);
    const [draft, setDraft] = useState<Config | null>(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [message, setMessage] = useState('');
    const alive = useRef(true);
    const savedRevision = useRef(0);

    useEffect(() => {
        alive.current = true;
        return () => { alive.current = false; };
    }, []);

    useEffect(() => {
        let disposed = false;
        api.load().then((result) => {
            if (disposed) { return; }
            setView(result);
            setDraft(result.config);
            savedRevision.current = result.revision;
        }).catch((cause: Error) => { if (!disposed) { setError(cause.message); } });
        const timer = window.setInterval(() => {
            api.load().then((result) => { if (!disposed) { setView(result); } }).catch(() => {
                if (!disposed) { setError('Could not refresh automation status. Reload before saving if the server remains unavailable.'); }
            });
        }, 30000);
        return () => { disposed = true; window.clearInterval(timer); };
    }, [api]);

    async function mutate(action: () => Promise<View>, notice: string, replaceDraft: boolean) {
        setBusy(true);
        setError('');
        setMessage('');
        try {
            const result = await action();
            if (!alive.current) { return; }
            setView(result);
            if (replaceDraft) {
                setDraft(result.config);
                savedRevision.current = result.revision;
            }
            setMessage(notice);
        } catch (cause) {
            if (alive.current) { setError(cause instanceof Error ? cause.message : 'Request failed.'); }
        } finally {
            if (alive.current) { setBusy(false); }
        }
    }

    function updateRule(index: number, change: Partial<Rule>) {
        if (!draft) { return; }
        setMessage('');
        setDraft({...draft, schedule: draft.schedule.map((rule, i) => i === index ? {...rule, ...change} : rule)});
    }

    const validation = draft ? validate(draft) : null;
    const dirty = draft && view && JSON.stringify(draft) !== JSON.stringify(view.config);

    return (
        <section className='scheduled-dnd' aria-label='DND schedule'>
            <style>{styles}</style>
            <p>Protect your quiet hours. Your schedule runs on the server, even when you are offline.</p>
            {error && <div role='alert' className='dnd-error'>{error} <button type='button' disabled={busy} onClick={() => void mutate(() => api.load(), 'Configuration reloaded.', true)}>Reload saved configuration</button></div>}
            {message && <div role='status' className='dnd-notice'>{message}</div>}
            {!draft || !view ? <p role='status'>Loading your schedule…</p> : <>
                <section className='dnd-status' aria-label='Current automation status' aria-live='polite'>
                    <strong>{automationLabels[view.automation] ?? 'Automation status unavailable.'}</strong>
                    <div>Mattermost status: {view.status}{view.dnd_end_time > 0 && view.status === 'dnd' ? ` until ${formatDate(new Date(view.dnd_end_time * 1000).toISOString(), view.config.timezone)}` : ''}</div>
                    {view.current && <div>Current interval ends {formatDate(view.current.end, view.config.timezone)}.</div>}
                    {view.next && <div>Next interval starts {formatDate(view.next.start, view.config.timezone)}.</div>}
                    <small>Times shown in {view.config.timezone}. Status refreshes every 30 seconds.</small>
                </section>
                <form onSubmit={(event) => {
                    event.preventDefault();
                    if (!validation) { void mutate(() => api.save(draft, savedRevision.current), 'Schedule saved. Changes are evaluated within one minute.', true); }
                }}>
                    <fieldset disabled={busy} className='dnd-fields'>
                        <label className='dnd-toggle'><input type='checkbox' role='switch' checked={draft.enabled} onChange={(event) => setDraft({...draft, enabled: event.currentTarget.checked})}/> Enable automatic DND</label>
                        <label htmlFor='dnd-timezone'>Timezone</label>
                        <input className='form-control' id='dnd-timezone' list='dnd-timezones' value={draft.timezone} maxLength={100} onChange={(event) => setDraft({...draft, timezone: event.currentTarget.value})} aria-describedby='dnd-timezone-help'/>
                        <datalist id='dnd-timezones'>{timezones().map((zone) => <option key={zone} value={zone}/>)}</datalist>
                        <small id='dnd-timezone-help'>Choose an IANA timezone. Daylight saving time is handled automatically.</small>
                        {draft.schedule.map((rule, index) => <fieldset key={index} className='dnd-rule'>
                            <legend>Interval {index + 1}</legend>
                            <div className='dnd-days'>{days.map((day) => <label key={day}><input type='checkbox' checked={rule.days.includes(day)} onChange={(event) => updateRule(index, {days: event.currentTarget.checked ? [...rule.days, day] : rule.days.filter((value) => value !== day)})}/>{day.slice(0, 3)}</label>)}</div>
                            <div className='dnd-times'>
                                <label>Start<input className='form-control' type='time' step={60} required value={rule.start} onChange={(event) => updateRule(index, {start: event.currentTarget.value})}/></label>
                                <label>End<input className='form-control' type='time' step={60} required value={rule.end} onChange={(event) => updateRule(index, {end: event.currentTarget.value})}/></label>
                                <button type='button' className='btn btn-tertiary' onClick={() => setDraft({...draft, schedule: draft.schedule.filter((_, i) => i !== index)})} aria-label={`Remove interval ${index + 1}`}>Remove</button>
                            </div>
                            {rule.end < rule.start && <small>Ends the following morning. Selected days are interval start days.</small>}
                        </fieldset>)}
                        <button className='btn btn-tertiary' type='button' disabled={draft.schedule.length >= 32} onClick={() => setDraft({...draft, schedule: [...draft.schedule, newRule()]})}>+ Add interval</button>
                        <section aria-label='Schedule summary'><h3>Schedule summary</h3>
                            {draft.schedule.length ? <ul>{summary(draft).map((line, index) => <li key={index}>{line}</li>)}</ul> : <p>No intervals configured.</p>}
                            <small>Days indicate when an interval starts. Overlapping and touching intervals merge. Equal start and end times are invalid.</small>
                        </section>
                        {validation && <p role='alert' className='dnd-error'>{validation}</p>}
                        <footer>
                            <button type='submit' className='btn btn-primary' disabled={Boolean(validation) || !dirty}>{busy ? 'Saving…' : 'Save schedule'}</button>
                            <button type='button' className='btn btn-tertiary' onClick={() => {
                                if (window.confirm('Reset your saved schedule and disable automation? Existing DND will expire naturally.')) {
                                    void mutate(() => api.reset(savedRevision.current), 'Configuration reset. Existing DND will expire naturally.', true);
                                }
                            }}>Reset configuration</button>
                        </footer>
                    </fieldset>
                </form>
                <hr/>
                <button type='button' className='btn btn-tertiary' disabled={busy || !view.current || view.automation === 'skipped'} onClick={() => void mutate(() => api.skip(), 'This interval is skipped. To leave active DND now, use your Mattermost status menu.', false)}>Skip current interval</button>
                <p className='dnd-help'>Skipping prevents activation. To end active DND, use your Mattermost status menu. Manual changes are respected for the rest of this interval. Saving edits after an interval was handled defers automation until the current interval ends; existing DND keeps its original expiration.</p>
            </>}
        </section>
    );
}
