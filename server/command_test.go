package main

import (
	"context"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestCommandsArePrivateAndIsolated(t *testing.T) {
	p, f := httpPlugin(t)
	config := configFor([]string{"monday"}, "17:00", "08:00")
	if err := p.service.Save(context.Background(), userID, config, 0); err != nil { t.Fatal(err) }
	for _, action := range []string{"off", "on"} {
		response, appErr := p.ExecuteCommand(nil, &model.CommandArgs{UserId: userID, Command: "/dnd-schedule " + action})
		if appErr != nil || response.ResponseType != model.CommandResponseTypeEphemeral { t.Fatal("command response is not private", appErr) }
		record, _, err := p.service.load(userID)
		if err != nil || record.Config.Enabled != (action == "on") { t.Fatal("enable/disable did not persist", record, err) }
	}
	for _, action := range []string{"status", "skip", "help", "unknown", "on extra"} {
		response, appErr := p.ExecuteCommand(nil, &model.CommandArgs{UserId: userID, Command: "/dnd-schedule " + action})
		if appErr != nil || response.ResponseType != model.CommandResponseTypeEphemeral { t.Fatal("command response is not private", appErr) }
	}
	record, _, err := p.service.load(userID)
	if err != nil || !record.Config.Enabled || record.Runtime.SuppressUntil == 0 { t.Fatal("enable/skip commands did not persist", record, err) }
	other, _, err := p.service.load(otherID)
	if err != nil || other.Revision != 0 { t.Fatal("another user's schedule changed") }
	if f.sets[userID] != 0 { t.Fatal("commands directly changed presence") }
	response, _ := p.ExecuteCommand(nil, &model.CommandArgs{UserId: otherID, Command: "/dnd-schedule status"})
	if strings.Contains(response.Text, "monday") { t.Fatal("schedule leaked to another user") }
}

func TestCommandValidation(t *testing.T) {
	p, f := httpPlugin(t)
	for _, args := range []*model.CommandArgs{nil, {UserId: "invalid"}, {UserId: "cccccccccccccccccccccccccc"}} {
		response, appErr := p.ExecuteCommand(nil, args)
		if appErr != nil || response == nil || response.ResponseType != model.CommandResponseTypeEphemeral { t.Fatal("invalid authentication response") }
	}
	response, _ := p.ExecuteCommand(nil, &model.CommandArgs{UserId: userID, Command: "/dnd-schedule on"})
	if !strings.Contains(response.Text, "interval") { t.Fatal("empty enabled schedule accepted", response.Text) }
	record, _, _ := p.service.load(userID)
	if record.Revision != 0 { t.Fatal("invalid command persisted configuration") }
	f.users[userID].DeleteAt = 1
	response, _ = p.ExecuteCommand(nil, &model.CommandArgs{UserId: userID, Command: "/dnd-schedule off"})
	if !strings.Contains(response.Text, "active user") { t.Fatal("deactivated user accepted") }
}
