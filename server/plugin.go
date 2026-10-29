package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

type Plugin struct {
	plugin.MattermostPlugin
	service *Service
	job *cluster.Job
	ctx context.Context
	cancel context.CancelFunc
}

func (p *Plugin) OnActivate() error {
	cfg := p.API.GetConfig()
	if cfg == nil || cfg.ServiceSettings.EnableUserStatuses == nil || !*cfg.ServiceSettings.EnableUserStatuses {
		return fmt.Errorf("Scheduled DND requires ServiceSettings.EnableUserStatuses")
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.service = &Service{api: p.API, now: time.Now, lock: func(ctx context.Context, id string) (func(), error) {
		mutex, err := cluster.NewMutex(p.API, "scheduled-dnd-user-"+id)
		if err != nil { return nil, err }
		if err := mutex.LockWithContext(ctx); err != nil { return nil, err }
		return mutex.Unlock, nil
	}}
	job, err := cluster.Schedule(p.API, "scheduled-dnd-tick", cluster.MakeWaitForRoundedInterval(30*time.Second), p.runJob)
	if err != nil { p.cancel(); return err }
	p.job = job
	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.cancel != nil { p.cancel() }
	if p.job != nil { return p.job.Close() }
	return nil
}

func (p *Plugin) runJob() {
	started := time.Now()
	// Snapshot the keys before acquiring per-user mutexes: these create transient
	// KV keys and must not perturb offset pagination during the scan.
	ids := map[string]bool{}
	for page := 0; ; page++ {
		if p.ctx.Err() != nil { return }
		keys, appErr := p.API.KVList(page, 200)
		if appErr != nil { p.API.LogError("Scheduled DND key scan failed", "error", appErr.Error()); return }
		for _, key := range keys {
			if strings.HasPrefix(key, recordPrefix) {
				id := strings.TrimPrefix(key, recordPrefix)
				if model.IsValidId(id) { ids[id] = true }
			}
		}
		if len(keys) < 200 { break }
	}
	for id := range ids {
		if p.ctx.Err() != nil { return }
		ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
		err := p.service.Evaluate(ctx, id)
		cancel()
		if err != nil { p.API.LogWarn("Scheduled DND evaluation failed", "user_id", id, "error", err.Error()) }
	}
	if time.Since(started) > time.Minute {
		p.API.LogWarn("Scheduled DND scan exceeded one minute", "users", len(ids))
	}
}
