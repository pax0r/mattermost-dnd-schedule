package main

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const userID = "aaaaaaaaaaaaaaaaaaaaaaaaaa"
const otherID = "bbbbbbbbbbbbbbbbbbbbbbbbbb"

type fakeAPI struct {
	sync.Mutex
	data map[string][]byte
	statuses map[string]*model.Status
	users map[string]*model.User
	sets map[string]int
	setFailure bool
	kvFailure bool
	statusScript []*model.Status
}

func newFake() *fakeAPI {
	return &fakeAPI{data: map[string][]byte{}, sets: map[string]int{},
		statuses: map[string]*model.Status{userID: {UserId: userID, Status: model.StatusOffline}, otherID: {UserId: otherID, Status: model.StatusOnline}},
		users: map[string]*model.User{userID: {Id: userID}, otherID: {Id: otherID}}}
}

func testError() *model.AppError {
	return model.NewAppError("fake", "test.error", nil, "injected failure", 500)
}

func (f *fakeAPI) KVGet(key string) ([]byte, *model.AppError) {
	f.Lock(); defer f.Unlock()
	if f.kvFailure { return nil, testError() }
	return bytes.Clone(f.data[key]), nil
}

func (f *fakeAPI) KVCompareAndSet(key string, old, value []byte) (bool, *model.AppError) {
	f.Lock(); defer f.Unlock()
	if f.kvFailure { return false, testError() }
	if !bytes.Equal(f.data[key], old) { return false, nil }
	f.data[key] = bytes.Clone(value)
	return true, nil
}

func (f *fakeAPI) KVList(page, perPage int) ([]string, *model.AppError) {
	f.Lock(); defer f.Unlock()
	keys := []string{}
	for key := range f.data { keys = append(keys, key) }
	sort.Strings(keys)
	start := min(page*perPage, len(keys))
	return keys[start:min(start+perPage, len(keys))], nil
}

func (f *fakeAPI) GetUser(id string) (*model.User, *model.AppError) {
	f.Lock(); defer f.Unlock()
	user := f.users[id]
	if user == nil { return nil, model.NewAppError("fake", "user.missing", nil, "", 404) }
	copy := *user
	return &copy, nil
}

func (f *fakeAPI) GetUserStatus(id string) (*model.Status, *model.AppError) {
	f.Lock(); defer f.Unlock()
	if len(f.statusScript) > 0 {
		status := *f.statusScript[0]
		f.statusScript = f.statusScript[1:]
		return &status, nil
	}
	status := f.statuses[id]
	if status == nil { return nil, model.NewAppError("fake", "status.missing", nil, "", 404) }
	copy := *status
	return &copy, nil
}

func (f *fakeAPI) SetUserStatusTimedDND(id string, end int64) (*model.Status, *model.AppError) {
	f.Lock(); defer f.Unlock()
	f.sets[id]++
	if f.setFailure { return nil, testError() }
	status := &model.Status{UserId: id, Status: model.StatusDnd, Manual: true, DNDEndTime: end}
	f.statuses[id] = status
	copy := *status
	return &copy, nil
}

func serviceFor(f *fakeAPI, now time.Time) *Service {
	// No process mutex: exercising CAS itself is stronger than serializing the fake.
	return &Service{api: f, now: func() time.Time { return now },
		lock: func(context.Context, string) (func(), error) { return func() {}, nil }}
}

func saveSchedule(t *testing.T, s *Service, id string) {
	t.Helper()
	if err := s.Save(context.Background(), id, configFor([]string{"monday"}, "17:00", "08:00"), 0); err != nil { t.Fatal(err) }
}

func TestRestartAndManualStatusChanges(t *testing.T) {
	ctx := context.Background()
	now := instant(t, "2026-10-26T18:00:00Z")
	for _, manual := range []string{model.StatusOnline, model.StatusAway, model.StatusOffline, model.StatusDnd} {
		t.Run(manual, func(t *testing.T) {
			f := newFake()
			s := serviceFor(f, now)
			saveSchedule(t, s, userID)
			if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
			end := instant(t, "2026-10-27T07:00:00Z").Unix()
			if f.sets[userID] != 1 || f.statuses[userID].DNDEndTime != end { t.Fatal("expected timed DND with epoch seconds") }
			f.statuses[userID] = &model.Status{Status: manual, Manual: true, DNDEndTime: end+3600}
			// A newly constructed service represents a restart or another HA node.
			restarted := serviceFor(f, now.Add(time.Minute))
			if err := restarted.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
			after := serviceFor(f, instant(t, "2026-10-27T07:01:00Z"))
			if err := after.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
			if f.sets[userID] != 1 || f.statuses[userID].Status != manual { t.Fatal("manual status overwritten") }
		})
	}
}

func TestExistingDNDAndManualAwayOfflineArePreserved(t *testing.T) {
	now := instant(t, "2026-10-26T18:00:00Z")
	for _, status := range []*model.Status{
		{Status: model.StatusDnd, DNDEndTime: 0},
		{Status: model.StatusDnd, DNDEndTime: now.Add(72*time.Hour).Unix()},
		{Status: model.StatusDnd, DNDEndTime: now.Add(time.Minute).Unix()},
		{Status: model.StatusAway, Manual: true},
		{Status: model.StatusOffline, Manual: true},
		{Status: "ooo", Manual: true},
	} {
		f := newFake()
		f.statuses[userID] = status
		s := serviceFor(f, now)
		saveSchedule(t, s, userID)
		if err := s.Evaluate(context.Background(), userID); err != nil { t.Fatal(err) }
		f.statuses[userID] = &model.Status{Status: model.StatusOnline}
		if err := s.Evaluate(context.Background(), userID); err != nil { t.Fatal(err) }
		if f.sets[userID] != 0 { t.Fatal("preexisting status or later manual change overwritten") }
	}
}

func TestRestartRecoversUnprocessedIntervalAndMultipleUsers(t *testing.T) {
	f := newFake()
	now := instant(t, "2026-10-26T18:00:00Z")
	s := serviceFor(f, now)
	saveSchedule(t, s, userID)
	saveSchedule(t, s, otherID)
	restarted := serviceFor(f, now.Add(time.Minute))
	for _, id := range []string{userID, otherID} {
		if err := restarted.Evaluate(context.Background(), id); err != nil { t.Fatal(err) }
		if f.sets[id] != 1 { t.Fatal("user not recovered independently") }
	}
}

func TestConcurrentNodesAndLeaderChanges(t *testing.T) {
	f := newFake()
	now := instant(t, "2026-10-26T18:00:00Z")
	s := serviceFor(f, now)
	saveSchedule(t, s, userID)
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			err := serviceFor(f, now).Evaluate(context.Background(), userID)
			if err != nil && !errors.Is(err, errConflict) { t.Error(err) }
		}()
	}
	workers.Wait()
	if f.sets[userID] != 1 { t.Fatalf("%d activations across nodes", f.sets[userID]) }
	if err := serviceFor(f, now.Add(2*time.Minute)).Evaluate(context.Background(), userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 1 { t.Fatal("new leader repeated activation") }
}

func TestActiveEditsSkipAndRevisionConflicts(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	s := serviceFor(f, instant(t, "2026-10-26T18:00:00Z"))
	saveSchedule(t, s, userID)
	if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	config := configFor([]string{"monday"}, "17:00", "10:00")
	if err := s.Save(ctx, userID, config, 1); err != nil { t.Fatal(err) }
	if !errors.Is(s.Save(ctx, userID, config, 1), errConflict) { t.Fatal("stale save accepted") }
	if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	f.statuses[userID] = &model.Status{Status: model.StatusOnline}
	afterOldEnd := serviceFor(f, instant(t, "2026-10-27T08:00:00Z"))
	if err := afterOldEnd.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 1 { t.Fatal("editing reactivated DND during extended interval") }
	if err := s.Skip(ctx, userID); err != nil { t.Fatal(err) }
	view, err := s.View(userID)
	if err != nil || view.Automation != "skipped" { t.Fatalf("skip not visible: %+v %v", view, err) }
}

func TestSkipBeforeFirstTickAndResetRetainsDecision(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	now := instant(t, "2026-10-26T18:00:00Z")
	s := serviceFor(f, now)
	saveSchedule(t, s, userID)
	if err := s.Skip(ctx, userID); err != nil { t.Fatal(err) }
	if err := serviceFor(f, now.Add(time.Minute)).Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 0 { t.Fatal("skipped interval activated") }
	if err := s.Save(ctx, userID, defaultConfig(), 1); err != nil { t.Fatal(err) }
	if err := s.Save(ctx, userID, configFor([]string{"monday"}, "17:00", "08:00"), 2); err != nil { t.Fatal(err) }
	if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 0 { t.Fatal("reset erased override") }
}

func TestFailuresAndAmbiguousCrashAreNotRetried(t *testing.T) {
	ctx := context.Background()
	now := instant(t, "2026-10-26T18:00:00Z")
	t.Run("status API fails", func(t *testing.T) {
		f := newFake(); f.setFailure = true
		s := serviceFor(f, now); saveSchedule(t, s, userID)
		if err := s.Evaluate(ctx, userID); err == nil { t.Fatal("expected failure") }
		f.setFailure = false
		if err := serviceFor(f, now).Evaluate(ctx, userID); err != nil { t.Fatal(err) }
		if f.sets[userID] != 1 { t.Fatal("ambiguous call retried") }
	})
	t.Run("crash after claim", func(t *testing.T) {
		f := newFake(); s := serviceFor(f, now); saveSchedule(t, s, userID)
		r, raw, _ := s.load(userID)
		in, _, _ := r.Config.CurrentAndNext(now)
		r.Runtime = Runtime{Start: in.Start.Unix(), End: in.End.Unix(), Outcome: "pending"}
		if _, err := s.cas(userID, raw, r); err != nil { t.Fatal(err) }
		if err := serviceFor(f, now).Evaluate(ctx, userID); err != nil { t.Fatal(err) }
		if f.sets[userID] != 0 { t.Fatal("repeated uncertain activation") }
	})
	t.Run("storage failure", func(t *testing.T) {
		f := newFake(); s := serviceFor(f, now); saveSchedule(t, s, userID); f.kvFailure = true
		if err := s.Evaluate(ctx, userID); err == nil { t.Fatal("storage error swallowed") }
		if f.sets[userID] != 0 { t.Fatal("status set without durable claim") }
	})
}

func TestStatusRecheckAndDeletedUsers(t *testing.T) {
	ctx := context.Background()
	now := instant(t, "2026-10-26T18:00:00Z")
	f := newFake()
	f.statusScript = []*model.Status{{Status: model.StatusOnline}, {Status: model.StatusDnd, DNDEndTime: now.Add(48*time.Hour).Unix()}}
	s := serviceFor(f, now); saveSchedule(t, s, userID)
	if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 0 { t.Fatal("new manual DND overwritten on recheck") }
	for _, user := range []*model.User{nil, {Id: userID, DeleteAt: 123}, {Id: userID, IsBot: true}} {
		f := newFake(); f.users[userID] = user
		s := serviceFor(f, now); saveSchedule(t, s, userID)
		if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
		if f.sets[userID] != 0 { t.Fatal("inactive user changed") }
	}
}

func TestMissingStatusAndDisabledConfig(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	delete(f.statuses, userID)
	s := serviceFor(f, instant(t, "2026-10-26T18:00:00Z"))
	if err := s.Save(ctx, userID, Config{Timezone: "UTC"}, 0); err != nil { t.Fatal(err) }
	view, err := s.View(userID)
	if err != nil || view.Status != model.StatusOffline || view.Config.Schedule == nil { t.Fatal("missing status/config defaults not handled", err) }
	if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 0 { t.Fatal("disabled schedule activated") }
	if err := s.Save(ctx, userID, configFor([]string{"monday"}, "17:00", "08:00"), 1); err != nil { t.Fatal(err) }
	if err := s.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 1 { t.Fatal("valid user with no status row did not activate") }
}

func TestFollowingOccurrenceAfterOverride(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	s := serviceFor(f, instant(t, "2026-10-26T18:00:00Z"))
	saveSchedule(t, s, userID)
	if err := s.Skip(ctx, userID); err != nil { t.Fatal(err) }
	nextWeek := serviceFor(f, instant(t, "2026-11-02T18:00:00Z"))
	if err := nextWeek.Evaluate(ctx, userID); err != nil { t.Fatal(err) }
	if f.sets[userID] != 1 || f.statuses[userID].DNDEndTime != instant(t, "2026-11-03T07:00:00Z").Unix() {
		t.Fatal("override leaked into the following occurrence")
	}
}
