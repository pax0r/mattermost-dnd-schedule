package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

var weekdays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

type Rule struct {
	Days []string `json:"days"`
	Start string `json:"start"`
	End string `json:"end"`
}

type Config struct {
	Enabled bool `json:"enabled"`
	Timezone string `json:"timezone"`
	Schedule []Rule `json:"schedule"`
}

func defaultConfig() Config {
	return Config{Timezone: "Europe/Warsaw", Schedule: []Rule{}}
}

func minuteOfDay(s string) (int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("time must use HH:mm")
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' { return 0, fmt.Errorf("time must use HH:mm") }
	}
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	if h > 23 || m > 59 { return 0, fmt.Errorf("time must be between 00:00 and 23:59") }
	return h*60 + m, nil
}

func dayNumber(day string) int {
	for i, name := range weekdays { if day == name { return i } }
	return -1
}

func (c Config) Validate() error {
	if c.Timezone == "" || c.Timezone == "Local" || len(c.Timezone) > 100 ||
		strings.Contains(c.Timezone, "..") || strings.HasPrefix(c.Timezone, "/") {
		return fmt.Errorf("choose an IANA timezone, such as Europe/Warsaw")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil { return fmt.Errorf("unknown IANA timezone") }
	if len(c.Schedule) > 32 { return fmt.Errorf("at most 32 intervals are allowed") }
	if c.Enabled && len(c.Schedule) == 0 { return fmt.Errorf("add at least one interval before enabling") }
	// A full weekly union has no finite end or temporary override boundary.
	var covered [7*24*60]bool
	for i, r := range c.Schedule {
		start, err := minuteOfDay(r.Start)
		if err != nil { return fmt.Errorf("interval %d start: %w", i+1, err) }
		end, err := minuteOfDay(r.End)
		if err != nil { return fmt.Errorf("interval %d end: %w", i+1, err) }
		if start == end { return fmt.Errorf("interval %d: start and end must differ", i+1) }
		if len(r.Days) == 0 || len(r.Days) > 7 { return fmt.Errorf("interval %d: choose one to seven start days", i+1) }
		seen := map[string]bool{}
		for _, day := range r.Days {
			d := dayNumber(day)
			if d < 0 || seen[day] { return fmt.Errorf("interval %d: invalid or duplicate weekday", i+1) }
			seen[day] = true
			duration := (end - start + 1440) % 1440
			for m := 0; m < duration; m++ { covered[(d*1440+start+m)%len(covered)] = true }
		}
	}
	for _, active := range covered { if !active { return nil } }
	return fmt.Errorf("schedule must include a weekly gap; continuous 24/7 DND is not supported")
}

type Interval struct {
	Start time.Time `json:"start"`
	End time.Time `json:"end"`
}

// resolveWall explicitly resolves folds and gaps instead of relying on time.Date's
// unspecified choice at DST boundaries. A fold uses the first start/last end;
// a missing minute advances to the first valid wall minute (up to one day).
func resolveWall(date time.Time, minute int, loc *time.Location, last bool) (time.Time, error) {
	wall := time.Date(date.Year(), date.Month(), date.Day(), minute/60, minute%60, 0, 0, time.UTC)
	offsets := map[int]bool{}
	for h := -48; h <= 48; h += 6 {
		_, offset := wall.Add(time.Duration(h)*time.Hour).In(loc).Zone()
		offsets[offset] = true
	}
	for step := 0; step <= 1440; step++ {
		target := wall.Add(time.Duration(step)*time.Minute)
		var chosen time.Time
		for offset := range offsets {
			candidate := target.Add(-time.Duration(offset)*time.Second)
			local := candidate.In(loc)
			if local.Year() != target.Year() || local.Month() != target.Month() || local.Day() != target.Day() ||
				local.Hour() != target.Hour() || local.Minute() != target.Minute() { continue }
			if chosen.IsZero() || (!last && candidate.Before(chosen)) || (last && candidate.After(chosen)) { chosen = candidate }
		}
		if !chosen.IsZero() { return chosen, nil }
	}
	return time.Time{}, fmt.Errorf("cannot resolve timezone boundary")
}

// intervals returns continuous unions, merging both overlaps and touching edges.
// days always names the local calendar day on which an interval starts.
func (c Config) intervals(now time.Time) ([]Interval, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil { return nil, err }
	local := now.In(loc)
	// UTC here is only a container for calendar dates; never add 24h in local time.
	date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	var all []Interval
	for d := -8; d <= 15; d++ {
		day := date.AddDate(0, 0, d)
		for _, r := range c.Schedule {
			selected := false
			for _, name := range r.Days { if dayNumber(name) == int(day.Weekday()) { selected = true } }
			if !selected { continue }
			startMin, err := minuteOfDay(r.Start)
			if err != nil { return nil, err }
			endMin, err := minuteOfDay(r.End)
			if err != nil { return nil, err }
			endDay := day
			if endMin <= startMin { endDay = endDay.AddDate(0, 0, 1) }
			start, err := resolveWall(day, startMin, loc, false)
			if err != nil { return nil, err }
			end, err := resolveWall(endDay, endMin, loc, true)
			if err != nil { return nil, err }
			// A spring gap can collapse an entire short interval to zero length.
			if end.After(start) { all = append(all, Interval{Start: start, End: end}) }
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	merged := []Interval{}
	for _, in := range all {
		if len(merged) == 0 || in.Start.After(merged[len(merged)-1].End) {
			merged = append(merged, in)
		} else if in.End.After(merged[len(merged)-1].End) {
			merged[len(merged)-1].End = in.End
		}
	}
	return merged, nil
}

func (c Config) CurrentAndNext(now time.Time) (*Interval, *Interval, error) {
	if !c.Enabled { return nil, nil, nil }
	all, err := c.intervals(now)
	if err != nil { return nil, nil, err }
	var active, next *Interval
	for _, in := range all {
		copy := in
		if !now.Before(in.Start) && now.Before(in.End) { active = &copy }
		if in.Start.After(now) { next = &copy; break }
	}
	return active, next, nil
}
