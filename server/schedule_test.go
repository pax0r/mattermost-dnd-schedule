package main

import (
	"testing"
	"time"
)

func instant(t *testing.T, value string) time.Time {
	t.Helper()
	now, err := time.Parse(time.RFC3339, value)
	if err != nil { t.Fatal(err) }
	return now
}

func configFor(days []string, start, end string) Config {
	return Config{Enabled: true, Timezone: "Europe/Warsaw", Schedule: []Rule{{Days: days, Start: start, End: end}}}
}

func TestScheduleBoundaries(t *testing.T) {
	tests := []struct {
		name string
		config Config
		now string
		start string
		end string
	}{
		{"daily start inclusive", configFor([]string{"monday"}, "09:00", "17:00"), "2026-10-26T08:00:00Z", "2026-10-26T08:00:00Z", "2026-10-26T16:00:00Z"},
		{"daily end exclusive", configFor([]string{"monday"}, "09:00", "17:00"), "2026-10-26T16:00:00Z", "", ""},
		{"overnight Monday", configFor([]string{"monday"}, "17:00", "08:00"), "2026-10-27T06:59:59Z", "2026-10-26T16:00:00Z", "2026-10-27T07:00:00Z"},
		{"overnight ended", configFor([]string{"monday"}, "17:00", "08:00"), "2026-10-27T07:00:00Z", "", ""},
		{"Friday extends to Saturday", configFor([]string{"friday"}, "17:00", "08:00"), "2026-10-31T06:00:00Z", "2026-10-30T16:00:00Z", "2026-10-31T07:00:00Z"},
		{"independent weekend", configFor([]string{"saturday", "sunday"}, "10:00", "12:00"), "2026-10-31T10:00:00Z", "2026-10-31T09:00:00Z", "2026-10-31T11:00:00Z"},
		{"Monday excluded", configFor([]string{"saturday", "sunday"}, "10:00", "12:00"), "2026-10-26T10:00:00Z", "", ""},
		{"spring overnight is shorter", configFor([]string{"saturday"}, "17:00", "08:00"), "2026-03-29T01:30:00Z", "2026-03-28T16:00:00Z", "2026-03-29T06:00:00Z"},
		{"autumn overnight is longer", configFor([]string{"saturday"}, "17:00", "08:00"), "2026-10-25T01:30:00Z", "2026-10-24T15:00:00Z", "2026-10-25T07:00:00Z"},
		{"spring missing start advances", configFor([]string{"sunday"}, "02:30", "04:00"), "2026-03-29T01:00:00Z", "2026-03-29T01:00:00Z", "2026-03-29T02:00:00Z"},
		{"spring missing end advances", configFor([]string{"sunday"}, "01:00", "02:30"), "2026-03-29T00:30:00Z", "2026-03-29T00:00:00Z", "2026-03-29T01:00:00Z"},
		{"spring collapsed interval skipped", configFor([]string{"sunday"}, "02:10", "02:50"), "2026-03-29T01:00:00Z", "", ""},
		{"autumn fold first occurrence", configFor([]string{"sunday"}, "02:15", "02:45"), "2026-10-25T00:20:00Z", "2026-10-25T00:15:00Z", "2026-10-25T01:45:00Z"},
		{"autumn fold second occurrence", configFor([]string{"sunday"}, "02:15", "02:45"), "2026-10-25T01:20:00Z", "2026-10-25T00:15:00Z", "2026-10-25T01:45:00Z"},
		{"autumn fold gap remains continuous", configFor([]string{"sunday"}, "02:15", "02:45"), "2026-10-25T00:55:00Z", "2026-10-25T00:15:00Z", "2026-10-25T01:45:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); err != nil { t.Fatal(err) }
			active, next, err := tt.config.CurrentAndNext(instant(t, tt.now))
			if err != nil { t.Fatal(err) }
			if tt.start == "" {
				if active != nil { t.Fatalf("unexpected active interval: %+v", active) }
			} else if active == nil || !active.Start.Equal(instant(t, tt.start)) || !active.End.Equal(instant(t, tt.end)) {
				t.Fatalf("got %+v, expected %s to %s", active, tt.start, tt.end)
			}
			if next == nil || !next.Start.After(instant(t, tt.now)) { t.Fatal("expected a future occurrence") }
		})
	}
}

func TestOverlapAndWeekBoundary(t *testing.T) {
	c := configFor([]string{"sunday"}, "23:00", "03:00")
	c.Schedule = append(c.Schedule, Rule{Days: []string{"monday"}, Start: "02:00", End: "05:00"}, Rule{Days: []string{"monday"}, Start: "05:00", End: "06:00"})
	active, _, err := c.CurrentAndNext(instant(t, "2026-10-26T02:00:00Z"))
	if err != nil { t.Fatal(err) }
	if active == nil || !active.Start.Equal(instant(t, "2026-10-25T22:00:00Z")) || !active.End.Equal(instant(t, "2026-10-26T05:00:00Z")) {
		t.Fatalf("expected merged interval across week boundary, got %+v", active)
	}
}

func TestDisabledAndTimezones(t *testing.T) {
	c := configFor([]string{"monday"}, "09:00", "17:00")
	c.Enabled = false
	a, n, err := c.CurrentAndNext(instant(t, "2026-10-26T12:00:00Z"))
	if err != nil || a != nil || n != nil { t.Fatal("disabled schedule must not activate") }
	c.Enabled = true
	c.Timezone = "America/New_York"
	a, _, err = c.CurrentAndNext(instant(t, "2026-10-26T12:00:00Z"))
	if err != nil || a != nil { t.Fatal("08:00 in New York is outside schedule") }
	a, _, err = c.CurrentAndNext(instant(t, "2026-10-26T14:00:00Z"))
	if err != nil || a == nil || a.End.Unix() != instant(t, "2026-10-26T21:00:00Z").Unix() { t.Fatal("timezone not applied") }
}

func TestMalformedSchedules(t *testing.T) {
	base := configFor([]string{"monday"}, "17:00", "08:00")
	cases := []Config{
		{Enabled: true, Timezone: "Europe/Warsaw"},
		{Timezone: "not/a-zone"}, {Timezone: "Local"}, {Timezone: "../UTC"}, {Timezone: ""},
		configFor([]string{"Monday"}, "17:00", "08:00"),
		configFor([]string{"monday", "monday"}, "17:00", "08:00"),
		configFor(nil, "17:00", "08:00"),
		configFor([]string{"monday"}, "24:00", "08:00"),
		configFor([]string{"monday"}, "7:00", "08:00"),
		configFor([]string{"monday"}, "+7:00", "08:00"),
		configFor([]string{"monday"}, "17:60", "08:00"),
		configFor([]string{"monday"}, "08:00", "08:00"),
	}
	for i, c := range cases { if c.Validate() == nil { t.Errorf("case %d accepted invalid config", i) } }
	base.Schedule = make([]Rule, 33)
	if base.Validate() == nil { t.Fatal("accepted too many rules") }
	continuous := configFor(weekdays, "00:00", "12:00")
	continuous.Schedule = append(continuous.Schedule, Rule{Days: weekdays, Start: "12:00", End: "00:00"})
	if continuous.Validate() == nil { t.Fatal("accepted a weekly union without an end") }
	if err := defaultConfig().Validate(); err != nil { t.Fatal(err) }
}
