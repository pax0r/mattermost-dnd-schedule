package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const recordPrefix = "user_"

var errConflict = errors.New("configuration changed; reload before saving")
var errNoInterval = errors.New("there is no current enabled interval to skip")

// api is deliberately limited to supported public plugin methods. Tests inject
// a fake implementing this interface, without a server or internal status store.
type api interface {
	KVGet(string) ([]byte, *model.AppError)
	KVCompareAndSet(string, []byte, []byte) (bool, *model.AppError)
	KVList(int, int) ([]string, *model.AppError)
	GetUser(string) (*model.User, *model.AppError)
	GetUserStatus(string) (*model.Status, *model.AppError)
	SetUserStatusTimedDND(string, int64) (*model.Status, *model.AppError)
}

type Runtime struct {
	Start int64 `json:"start"`
	End int64 `json:"end"`
	Outcome string `json:"outcome"`
	SuppressUntil int64 `json:"suppress_until"`
}

type Record struct {
	Config Config `json:"config"`
	Revision uint64 `json:"revision"`
	Runtime Runtime `json:"runtime"`
}

type View struct {
	Record
	Current *Interval `json:"current"`
	Next *Interval `json:"next"`
	Status string `json:"status"`
	DNDEndTime int64 `json:"dnd_end_time"`
	Automation string `json:"automation"`
	ServerTime time.Time `json:"server_time"`
}

type lockUser func(context.Context, string) (func(), error)

type Service struct {
	api api
	lock lockUser
	now func() time.Time
}

func (s *Service) load(id string) (Record, []byte, error) {
	raw, appErr := s.api.KVGet(recordPrefix + id)
	if appErr != nil { return Record{}, nil, appErr }
	record := Record{Config: defaultConfig()}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &record); err != nil { return Record{}, nil, fmt.Errorf("invalid persisted record: %w", err) }
		if err := record.Config.Validate(); err != nil { return Record{}, nil, fmt.Errorf("invalid persisted configuration: %w", err) }
	}
	if record.Config.Schedule == nil { record.Config.Schedule = []Rule{} }
	return record, raw, nil
}

func (s *Service) cas(id string, old []byte, record Record) ([]byte, error) {
	raw, err := json.Marshal(record)
	if err != nil { return nil, err }
	ok, appErr := s.api.KVCompareAndSet(recordPrefix+id, old, raw)
	if appErr != nil { return nil, appErr }
	if !ok { return nil, errConflict }
	return raw, nil
}

func (s *Service) Save(ctx context.Context, id string, config Config, revision uint64) error {
	if err := config.Validate(); err != nil { return err }
	if config.Schedule == nil { config.Schedule = []Rule{} }
	unlock, err := s.lock(ctx, id)
	if err != nil { return err }
	defer unlock()
	record, raw, err := s.load(id)
	if err != nil { return err }
	if record.Revision != revision { return errConflict }
	// Keep a decision made for a currently running interval across edits, resets,
	// disabling, and reenabling. Never reinterpret a status as plugin-owned.
	now := s.now()
	if record.Runtime.End > now.Unix() || record.Runtime.SuppressUntil > now.Unix() {
		current, _, err := config.CurrentAndNext(now)
		if err != nil { return err }
		until := max(record.Runtime.End, record.Runtime.SuppressUntil)
		if current != nil { until = max(until, current.End.Unix()) }
		record.Runtime.SuppressUntil = until
	}
	record.Config = config
	record.Revision++
	_, err = s.cas(id, raw, record)
	return err
}

func (s *Service) Skip(ctx context.Context, id string) error {
	unlock, err := s.lock(ctx, id)
	if err != nil { return err }
	defer unlock()
	record, raw, err := s.load(id)
	if err != nil { return err }
	current, _, err := record.Config.CurrentAndNext(s.now())
	if err != nil { return err }
	if current == nil { return errNoInterval }
	record.Runtime.SuppressUntil = max(record.Runtime.SuppressUntil, current.End.Unix())
	_, err = s.cas(id, raw, record)
	return err
}

func eligible(status *model.Status) bool {
	if status == nil || status.Status == model.StatusDnd { return false }
	if status.Status == model.StatusOnline { return true }
	return !status.Manual && (status.Status == model.StatusAway || status.Status == model.StatusOffline)
}

func (s *Service) status(id string) (*model.Status, *model.AppError) {
	status, appErr := s.api.GetUserStatus(id)
	// A valid user may not have a status row yet. Mattermost's timed-DND setter
	// itself initializes that case from automatic offline status.
	if appErr != nil && appErr.StatusCode == 404 {
		return &model.Status{UserId: id, Status: model.StatusOffline}, nil
	}
	return status, appErr
}

func (s *Service) Evaluate(ctx context.Context, id string) error {
	unlock, err := s.lock(ctx, id)
	if err != nil { return err }
	defer unlock()
	record, raw, err := s.load(id)
	if err != nil { return err }
	now := s.now()
	if !record.Config.Enabled || record.Runtime.End > now.Unix() || record.Runtime.SuppressUntil > now.Unix() { return nil }
	current, _, err := record.Config.CurrentAndNext(now)
	if err != nil || current == nil { return err }
	user, appErr := s.api.GetUser(id)
	if appErr != nil {
		if appErr.StatusCode == 404 { return nil }
		return appErr
	}
	if user == nil || user.DeleteAt != 0 || user.IsBot { return nil }
	status, appErr := s.status(id)
	if appErr != nil { return appErr }
	// Persist before touching status. Pending and failed decisions are never
	// retried in this interval, including after a crash or leader change.
	record.Runtime = Runtime{Start: current.Start.Unix(), End: current.End.Unix(), Outcome: "preserved"}
	if eligible(status) { record.Runtime.Outcome = "pending" }
	claimed, err := s.cas(id, raw, record)
	if err != nil { return err }
	if record.Runtime.Outcome != "pending" { return nil }
	// Recheck both the durable record and live status immediately before writing.
	_, latest, err := s.load(id)
	if err != nil { return err }
	if string(latest) != string(claimed) { return errConflict }
	status, appErr = s.status(id)
	if appErr != nil { return appErr }
	if !eligible(status) {
		record.Runtime.Outcome = "preserved"
	} else if !s.now().Before(current.End) {
		record.Runtime.Outcome = "expired"
	} else {
		status, appErr = s.api.SetUserStatusTimedDND(id, current.End.Unix())
		if appErr != nil || status == nil || status.Status != model.StatusDnd || status.DNDEndTime != current.End.Unix() {
			record.Runtime.Outcome = "failed"
		} else {
			record.Runtime.Outcome = "activated"
		}
	}
	_, err = s.cas(id, claimed, record)
	if appErr != nil { return appErr }
	return err
}

func (s *Service) View(id string) (View, error) {
	record, _, err := s.load(id)
	if err != nil { return View{}, err }
	now := s.now()
	current, next, err := record.Config.CurrentAndNext(now)
	if err != nil { return View{}, err }
	status, appErr := s.status(id)
	if appErr != nil { return View{}, appErr }
	if status == nil { return View{}, fmt.Errorf("status unavailable") }
	view := View{Record: record, Current: current, Next: next, Status: status.Status,
		DNDEndTime: status.DNDEndTime, Automation: "waiting", ServerTime: now}
	switch {
	case !record.Config.Enabled:
		view.Automation = "disabled"
	case record.Runtime.SuppressUntil > now.Unix():
		view.Automation = "skipped"
	case current == nil:
		view.Automation = "waiting"
	case record.Runtime.End <= now.Unix():
		view.Automation = "due"
	case record.Runtime.Outcome == "activated":
		view.Automation = "status_changed"
		if status.Status == model.StatusDnd && status.DNDEndTime == record.Runtime.End { view.Automation = "dnd_matches_schedule" }
	default:
		view.Automation = record.Runtime.Outcome
	}
	return view, nil
}
