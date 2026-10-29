export const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'] as const;
export type Day = typeof days[number];
export type Rule = {days: Day[]; start: string; end: string};
export type Config = {enabled: boolean; timezone: string; schedule: Rule[]};
export type Interval = {start: string; end: string};
export type View = {
    config: Config;
    revision: number;
    runtime: {start: number; end: number; outcome: string; suppress_until: number};
    current: Interval | null;
    next: Interval | null;
    status: string;
    dnd_end_time: number;
    automation: string;
    server_time: string;
};

export function newRule(): Rule {
    return {days: ['monday', 'tuesday', 'wednesday', 'thursday', 'friday'], start: '17:00', end: '08:00'};
}

export function validate(config: Config): string | null {
    if (!config.timezone || config.timezone === 'Local' || config.timezone.length > 100) {
        return 'Choose an IANA timezone, such as Europe/Warsaw.';
    }
    try {
        new Intl.DateTimeFormat('en', {timeZone: config.timezone}).format();
    } catch {
        return 'Unknown timezone. Use an IANA identifier such as Europe/Warsaw.';
    }
    if (config.enabled && config.schedule.length === 0) {
        return 'Add an interval before enabling automatic DND.';
    }
    if (config.schedule.length > 32) {
        return 'Use at most 32 intervals.';
    }
    const covered = new Set<number>();
    for (const [i, rule] of config.schedule.entries()) {
        if (rule.days.length === 0 || rule.days.some((day) => !days.includes(day)) || new Set(rule.days).size !== rule.days.length) {
            return `Interval ${i + 1}: choose valid, distinct start days.`;
        }
        if (![rule.start, rule.end].every((value) => /^(?:[01]\d|2[0-3]):[0-5]\d$/.test(value))) {
            return `Interval ${i + 1}: enter start and end as HH:mm.`;
        }
        if (rule.start === rule.end) {
            return `Interval ${i + 1}: start and end must differ.`;
        }
        const minutes = (s: string) => Number(s.slice(0, 2)) * 60 + Number(s.slice(3));
        const start = minutes(rule.start);
        const duration = (minutes(rule.end) - start + 1440) % 1440;
        for (const day of rule.days) {
            for (let offset = 0; offset < duration; offset++) {
                covered.add((days.indexOf(day) * 1440 + start + offset) % 10080);
            }
        }
    }
    if (covered.size === 10080) {
        return 'Include a weekly gap; continuous 24/7 DND has no interval end.';
    }
    return null;
}

export function summary(config: Config): string[] {
    return config.schedule.map((rule) => `${rule.days.map((day) => day.slice(0, 3)).join(', ')}: ${rule.start}–${rule.end}${rule.end < rule.start ? ' next day' : ''} (${config.timezone})`);
}

export const automationLabels: Record<string, string> = {
    disabled: 'Automation is disabled.',
    waiting: 'Waiting for the next interval.',
    due: 'This interval is due. The scheduler will evaluate it shortly.',
    skipped: 'Automation is paused for this interval.',
    pending: 'Activation was reserved; its result is unconfirmed. This interval will not be retried.',
    failed: 'Activation failed or was unconfirmed. This interval will not be retried.',
    expired: 'The interval ended before activation.',
    preserved: 'Your existing status was preserved for this interval.',
    status_changed: 'Status changed after activation. Automation will not reapply DND this interval.',
    dnd_matches_schedule: 'DND is active with the scheduled expiration.',
};
