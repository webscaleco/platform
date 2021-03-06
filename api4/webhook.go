// Copyright (c) 2017 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package api4

import (
	"net/http"

	l4g "github.com/alecthomas/log4go"
	"github.com/mattermost/platform/app"
	"github.com/mattermost/platform/model"
	"github.com/mattermost/platform/utils"
)

func InitWebhook() {
	l4g.Debug(utils.T("api.webhook.init.debug"))

	BaseRoutes.IncomingHooks.Handle("", ApiSessionRequired(createIncomingHook)).Methods("POST")
	BaseRoutes.IncomingHooks.Handle("", ApiSessionRequired(getIncomingHooks)).Methods("GET")
	BaseRoutes.IncomingHook.Handle("", ApiSessionRequired(getIncomingHook)).Methods("GET")
	BaseRoutes.IncomingHook.Handle("", ApiSessionRequired(updateIncomingHook)).Methods("PUT")
	BaseRoutes.IncomingHook.Handle("", ApiSessionRequired(deleteIncomingHook)).Methods("DELETE")

	BaseRoutes.OutgoingHooks.Handle("", ApiSessionRequired(createOutgoingHook)).Methods("POST")
	BaseRoutes.OutgoingHooks.Handle("", ApiSessionRequired(getOutgoingHooks)).Methods("GET")
	BaseRoutes.OutgoingHook.Handle("/regen_token", ApiSessionRequired(regenOutgoingHookToken)).Methods("POST")
}

func createIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	hook := model.IncomingWebhookFromJson(r.Body)
	if hook == nil {
		c.SetInvalidParam("incoming_webhook")
		return
	}

	channel, err := app.GetChannel(hook.ChannelId)
	if err != nil {
		c.Err = err
		return
	}

	c.LogAudit("attempt")

	if !app.SessionHasPermissionToTeam(c.Session, channel.TeamId, model.PERMISSION_MANAGE_WEBHOOKS) {
		c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
		return
	}

	if channel.Type != model.CHANNEL_OPEN && !app.SessionHasPermissionToChannel(c.Session, channel.Id, model.PERMISSION_READ_CHANNEL) {
		c.LogAudit("fail - bad channel permissions")
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	if incomingHook, err := app.CreateIncomingWebhookForChannel(c.Session.UserId, channel, hook); err != nil {
		c.Err = err
		return
	} else {
		c.LogAudit("success")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(incomingHook.ToJson()))
	}
}

func updateIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookId()
	if c.Err != nil {
		return
	}

	hookID := c.Params.HookId

	updatedHook := model.IncomingWebhookFromJson(r.Body)
	if updatedHook == nil {
		c.SetInvalidParam("incoming_webhook")
		return
	}

	c.LogAudit("attempt")

	oldHook, err := app.GetIncomingWebhook(hookID)
	if err != nil {
		c.Err = err
		return
	}

	if updatedHook.TeamId != oldHook.TeamId {
		c.Err = model.NewAppError("updateIncomingHook", "api.webhook.team_mismatch.app_error", nil, "user_id="+c.Session.UserId, http.StatusBadRequest)
		return
	}

	if !app.SessionHasPermissionToTeam(c.Session, updatedHook.TeamId, model.PERMISSION_MANAGE_WEBHOOKS) {
		c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
		return
	}

	if c.Session.UserId != updatedHook.UserId && !app.SessionHasPermissionToTeam(c.Session, updatedHook.TeamId, model.PERMISSION_MANAGE_OTHERS_WEBHOOKS) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PERMISSION_MANAGE_OTHERS_WEBHOOKS)
		return
	}

	channel, err := app.GetChannel(updatedHook.ChannelId)
	if err != nil {
		c.Err = err
		return
	}

	if channel.Type != model.CHANNEL_OPEN && !app.SessionHasPermissionToChannel(c.Session, channel.Id, model.PERMISSION_READ_CHANNEL) {
		c.LogAudit("fail - bad channel permissions")
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	if incomingHook, err := app.UpdateIncomingWebhook(oldHook, updatedHook); err != nil {
		c.Err = err
		return
	} else {
		c.LogAudit("success")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(incomingHook.ToJson()))
	}
}

func getIncomingHooks(c *Context, w http.ResponseWriter, r *http.Request) {
	teamId := r.URL.Query().Get("team_id")

	var hooks []*model.IncomingWebhook
	var err *model.AppError

	if len(teamId) > 0 {
		if !app.SessionHasPermissionToTeam(c.Session, teamId, model.PERMISSION_MANAGE_WEBHOOKS) {
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		}

		hooks, err = app.GetIncomingWebhooksForTeamPage(teamId, c.Params.Page, c.Params.PerPage)
	} else {
		if !app.SessionHasPermissionTo(c.Session, model.PERMISSION_MANAGE_WEBHOOKS) {
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		}

		hooks, err = app.GetIncomingWebhooksPage(c.Params.Page, c.Params.PerPage)
	}

	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.IncomingWebhookListToJson(hooks)))
}

func getIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookId()
	if c.Err != nil {
		return
	}

	hookID := c.Params.HookId

	var err *model.AppError
	var hook *model.IncomingWebhook
	var channel *model.Channel

	if hook, err = app.GetIncomingWebhook(hookID); err != nil {
		c.Err = err
		return
	} else {
		channel, err = app.GetChannel(hook.ChannelId)
		if err != nil {
			c.Err = err
			return
		}

		if !app.SessionHasPermissionToTeam(c.Session, hook.TeamId, model.PERMISSION_MANAGE_WEBHOOKS) ||
			(channel.Type != model.CHANNEL_OPEN && !app.SessionHasPermissionToChannel(c.Session, hook.ChannelId, model.PERMISSION_READ_CHANNEL)) {
			c.LogAudit("fail - bad permissions")
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		} else {
			w.Write([]byte(hook.ToJson()))
			return
		}
	}
}

func deleteIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookId()
	if c.Err != nil {
		return
	}

	hookID := c.Params.HookId

	var err *model.AppError
	var hook *model.IncomingWebhook
	var channel *model.Channel

	if hook, err = app.GetIncomingWebhook(hookID); err != nil {
		c.Err = err
		return
	} else {
		channel, err = app.GetChannel(hook.ChannelId)
		if err != nil {
			c.Err = err
			return
		}

		if !app.SessionHasPermissionToTeam(c.Session, hook.TeamId, model.PERMISSION_MANAGE_WEBHOOKS) ||
			(channel.Type != model.CHANNEL_OPEN && !app.SessionHasPermissionToChannel(c.Session, hook.ChannelId, model.PERMISSION_READ_CHANNEL)) {
			c.LogAudit("fail - bad permissions")
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		} else {
			if err = app.DeleteIncomingWebhook(hookID); err != nil {
				c.Err = err
				return
			}

			ReturnStatusOK(w)
		}
	}
}

func createOutgoingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	hook := model.OutgoingWebhookFromJson(r.Body)
	if hook == nil {
		c.SetInvalidParam("outgoing_webhook")
		return
	}

	c.LogAudit("attempt")

	hook.CreatorId = c.Session.UserId

	if !app.SessionHasPermissionToTeam(c.Session, hook.TeamId, model.PERMISSION_MANAGE_WEBHOOKS) {
		c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
		return
	}

	if rhook, err := app.CreateOutgoingWebhook(hook); err != nil {
		c.LogAudit("fail")
		c.Err = err
		return
	} else {
		c.LogAudit("success")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(rhook.ToJson()))
	}
}

func getOutgoingHooks(c *Context, w http.ResponseWriter, r *http.Request) {
	channelId := r.URL.Query().Get("channel_id")
	teamId := r.URL.Query().Get("team_id")

	var hooks []*model.OutgoingWebhook
	var err *model.AppError

	if len(channelId) > 0 {
		if !app.SessionHasPermissionToChannel(c.Session, channelId, model.PERMISSION_MANAGE_WEBHOOKS) {
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		}

		hooks, err = app.GetOutgoingWebhooksForChannelPage(channelId, c.Params.Page, c.Params.PerPage)
	} else if len(teamId) > 0 {
		if !app.SessionHasPermissionToTeam(c.Session, teamId, model.PERMISSION_MANAGE_WEBHOOKS) {
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		}

		hooks, err = app.GetOutgoingWebhooksForTeamPage(teamId, c.Params.Page, c.Params.PerPage)
	} else {
		if !app.SessionHasPermissionTo(c.Session, model.PERMISSION_MANAGE_WEBHOOKS) {
			c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
			return
		}

		hooks, err = app.GetOutgoingWebhooksPage(c.Params.Page, c.Params.PerPage)
	}

	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.OutgoingWebhookListToJson(hooks)))
}

func regenOutgoingHookToken(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookId()
	if c.Err != nil {
		return
	}

	hook, err := app.GetOutgoingWebhook(c.Params.HookId)
	if err != nil {
		c.Err = err
		return
	}

	c.LogAudit("attempt")

	if !app.SessionHasPermissionToTeam(c.Session, hook.TeamId, model.PERMISSION_MANAGE_WEBHOOKS) {
		c.SetPermissionError(model.PERMISSION_MANAGE_WEBHOOKS)
		return
	}

	if c.Session.UserId != hook.CreatorId && !app.SessionHasPermissionToTeam(c.Session, hook.TeamId, model.PERMISSION_MANAGE_OTHERS_WEBHOOKS) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PERMISSION_MANAGE_OTHERS_WEBHOOKS)
		return
	}

	if rhook, err := app.RegenOutgoingWebhookToken(hook); err != nil {
		c.Err = err
		return
	} else {
		w.Write([]byte(rhook.ToJson()))
	}
}
