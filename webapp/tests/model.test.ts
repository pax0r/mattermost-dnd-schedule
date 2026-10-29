import assert from 'node:assert/strict';
import test from 'node:test';

import {days, newRule, summary, validate} from '../src/model.ts';
import type {Config} from '../src/model.ts';

const config = (): Config => ({enabled: true, timezone: 'Europe/Warsaw', schedule: [newRule()]});

test('weekday overnight schedule has an explicit next-day summary', () => {
    const value = config();
    assert.equal(validate(value), null);
    assert.deepEqual(summary(value), ['mon, tue, wed, thu, fri: 17:00–08:00 next day (Europe/Warsaw)']);
});

test('independent weekend and multiple intervals', () => {
    const value = config();
    value.schedule.push({days: ['saturday', 'sunday'], start: '10:00', end: '12:00'});
    assert.equal(validate(value), null);
    assert.equal(summary(value)[1], 'sat, sun: 10:00–12:00 (Europe/Warsaw)');
});

test('disabled empty configuration is valid', () => {
    assert.equal(validate({enabled: false, timezone: 'UTC', schedule: []}), null);
    assert.match(validate({enabled: true, timezone: 'UTC', schedule: []})!, /Add an interval/);
});

test('timezone validation', () => {
    for (const timezone of ['', 'Local', 'Europe/Nonexistent']) {
        assert.match(validate({...config(), timezone})!, /timezone/i);
    }
    assert.equal(validate({...config(), timezone: 'America/New_York'}), null);
});

test('malformed times and empty weekdays have helpful messages', () => {
    for (const start of ['7:00', '24:00', '17:60', '', '08:00']) {
        const value = config();
        value.schedule[0].start = start;
        assert.ok(validate(value));
    }
    const value = config();
    value.schedule[0].days = [];
    assert.match(validate(value)!, /start days/);
    value.schedule[0].days = ['monday', 'monday'];
    assert.match(validate(value)!, /distinct/);
});

test('overlaps are accepted; a continuous weekly union is rejected', () => {
    const value = config();
    value.schedule.push(newRule());
    assert.equal(validate(value), null);
    value.schedule = [{days: [...days], start: '00:00', end: '12:00'}, {days: [...days], start: '12:00', end: '00:00'}];
    assert.match(validate(value)!, /weekly gap/);
});

test('rules are independently editable and bounded', () => {
    const first = newRule();
    const second = newRule();
    first.days.pop();
    assert.equal(second.days.length, 5);
    const value = config();
    value.schedule = Array.from({length: 33}, newRule);
    assert.match(validate(value)!, /32/);
});
