package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const commandHelp = "Open Settings → Scheduled DND to edit your schedule. Commands: `/dnd-schedule status`, `on`, `off`, `skip`, `help`. Skipping or disabling automation leaves existing DND to expire; use the Mattermost status menu to end it now."

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	reply := func(text string) (*model.CommandResponse, *model.AppError) {
		return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}, nil
	}
	if args == nil || !model.IsValidId(args.UserId) { return reply("Authentication required.") }
	user, appErr := p.API.GetUser(args.UserId)
	if appErr != nil || user == nil || user.DeleteAt != 0 || user.IsBot { return reply("An active user is required.") }
	parts := strings.Fields(args.Command)
	if len(parts) == 0 || parts[0] != "/dnd-schedule" || len(parts) > 2 { return reply(commandHelp) }
	action := "status"
	if len(parts) == 2 { action = strings.ToLower(parts[1]) }
	if action == "help" { return reply(commandHelp) }
	if action != "status" && action != "on" && action != "off" && action != "skip" { return reply(commandHelp) }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	switch action {
	case "on", "off":
		var record Record
		record, _, err = p.service.load(args.UserId)
		if err == nil {
			record.Config.Enabled = action == "on"
			if validation := record.Config.Validate(); validation != nil { return reply(validation.Error()+"\n"+commandHelp) }
			err = p.service.Save(ctx, args.UserId, record.Config, record.Revision)
		}
	case "skip":
		err = p.service.Skip(ctx, args.UserId)
	}
	if err != nil {
		p.API.LogWarn("Scheduled DND command failed", "error", err.Error())
		if err == errConflict || err == errNoInterval { return reply(err.Error()) }
		return reply("Request failed. Try again or ask an administrator to check the server logs.")
	}
	view, err := p.service.View(args.UserId)
	if err != nil { return reply("Automation status is unavailable. Try again later.") }
	var text strings.Builder
	fmt.Fprintf(&text, "Scheduled DND: enabled=%t; timezone=%s; automation=%s; Mattermost status=%s.\n", view.Config.Enabled, view.Config.Timezone, view.Automation, view.Status)
	for _, rule := range view.Config.Schedule {
		fmt.Fprintf(&text, "- %s: %s–%s (start days)\n", strings.Join(rule.Days, ", "), rule.Start, rule.End)
	}
	text.WriteString(commandHelp)
	return reply(text.String())
}
