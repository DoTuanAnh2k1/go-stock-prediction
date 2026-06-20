package handlers

import (
	"context"
	"net/url"

	"go-stock-prediction/cli-svc/internal/client"
)

func marketArg(required bool) ArgSpec {
	return ArgSpec{Name: "market", Required: required, Type: "string", Choices: Markets}
}

// defaultHandlers returns the initial catalog (spec §5).
func defaultHandlers() []Handler {
	return []Handler{
		// ---- GET ----
		{
			Key: "market.latest", DisplayName: "Latest market price", Verb: "get", Resource: "market.latest",
			ArgSchema: []ArgSpec{marketArg(true)},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/"+a["market"]+"/latest", nil)
			},
			render: autoRender("Latest price"),
		},
		{
			Key: "market.prices", DisplayName: "Market prices", Verb: "get", Resource: "market.prices",
			ArgSchema: []ArgSpec{marketArg(true), {Name: "limit", Required: false, Type: "int"}},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				q := url.Values{}
				if a["limit"] != "" {
					q.Set("limit", a["limit"])
				}
				return getJSON(ctx, c, jwt, "/"+a["market"]+"/prices", q)
			},
			render: autoRender("Prices"),
		},
		{
			Key: "market.predictions", DisplayName: "Latest predictions", Verb: "get", Resource: "market.predictions",
			ArgSchema: []ArgSpec{marketArg(true)},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/"+a["market"]+"/predictions/latest", nil)
			},
			render: autoRender("Predictions"),
		},
		{
			Key: "direction.accuracy", DisplayName: "Direction accuracy", Verb: "get", Resource: "direction.accuracy",
			ArgSchema: []ArgSpec{{Name: "market", Required: true, Type: "string", Choices: []string{"GOLD", "NASDAQ", "CRYPTO", "SP500"}}},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				q := url.Values{}
				q.Set("market", a["market"])
				return getJSON(ctx, c, jwt, "/predictions/direction-accuracy", q)
			},
			render: autoRender("Direction accuracy"),
		},
		{
			Key: "monitoring.overview", DisplayName: "Monitoring overview", Verb: "get", Resource: "monitoring.overview",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/monitoring/overview", nil)
			},
			render: autoRender("Monitoring"),
		},
		{
			Key: "schedules.list", DisplayName: "Cron schedules", Verb: "get", Resource: "schedules.list",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/schedules", nil)
			},
			render: autoRender("Schedules"),
		},
		{
			Key: "pipeline.reports", DisplayName: "Pipeline reports", Verb: "get", Resource: "pipeline.reports",
			ArgSchema: []ArgSpec{{Name: "pipeline", Required: false, Type: "string"}, {Name: "limit", Required: false, Type: "int"}},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				q := url.Values{}
				if a["pipeline"] != "" {
					q.Set("pipeline", a["pipeline"])
				}
				if a["limit"] != "" {
					q.Set("limit", a["limit"])
				}
				return getJSON(ctx, c, jwt, "/pipeline-reports", q)
			},
			render: autoRender("Pipeline reports"),
		},
		{
			Key: "training.status", DisplayName: "Training status", Verb: "get", Resource: "training.status",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/training/status", nil)
			},
			render: autoRender("Training status"),
		},
		{
			Key: "users.list", DisplayName: "Users", Verb: "get", Resource: "users.list",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/users", nil)
			},
			render: autoRender("Users"),
		},
		{
			Key: "backups.list", DisplayName: "Backups", Verb: "get", Resource: "backups.list",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return getJSON(ctx, c, jwt, "/backups", nil)
			},
			render: autoRender("Backups"),
		},

		// ---- SET (POST) ----
		{
			Key: "trigger.train", DisplayName: "Trigger training", Verb: "set", Resource: "trigger.train",
			ArgSchema: []ArgSpec{{Name: "algorithm", Required: false, Type: "string"}},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				var body any
				if a["algorithm"] != "" {
					body = map[string]string{"algorithm": a["algorithm"]}
				}
				return postJSON(ctx, c, jwt, "/trigger/train", body)
			},
			render: autoRender("Trigger train"),
		},
		{
			Key: "trigger.crawler", DisplayName: "Trigger crawler", Verb: "set", Resource: "trigger.crawler",
			ArgSchema: []ArgSpec{marketArg(true)},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return postJSON(ctx, c, jwt, "/trigger/"+a["market"]+"-crawler", nil)
			},
			render: autoRender("Trigger crawler"),
		},
		{
			Key: "trigger.predict", DisplayName: "Trigger predict", Verb: "set", Resource: "trigger.predict",
			ArgSchema: []ArgSpec{marketArg(true)},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return postJSON(ctx, c, jwt, "/trigger/"+a["market"]+"-predict", nil)
			},
			render: autoRender("Trigger predict"),
		},
		{
			Key: "trigger.reconcile", DisplayName: "Trigger reconcile", Verb: "set", Resource: "trigger.reconcile",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return postJSON(ctx, c, jwt, "/trigger/reconcile", nil)
			},
			render: autoRender("Trigger reconcile"),
		},
		{
			Key: "trigger.backup", DisplayName: "Trigger backup", Verb: "set", Resource: "trigger.backup",
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return postJSON(ctx, c, jwt, "/trigger/backup", nil)
			},
			render: autoRender("Trigger backup"),
		},

		// ---- UPDATE (PUT) ----
		{
			Key: "schedule.update", DisplayName: "Update schedule", Verb: "update", Resource: "schedule.update",
			ArgSchema: []ArgSpec{
				{Name: "key", Required: true, Type: "string"},
				{Name: "cron_expression", Required: true, Type: "string"},
				{Name: "enabled", Required: false, Type: "string", Choices: []string{"true", "false"}},
			},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				body := map[string]any{"cron_expression": a["cron_expression"]}
				if a["enabled"] != "" {
					body["enabled"] = a["enabled"] == "true"
				}
				return putJSON(ctx, c, jwt, "/schedules/"+a["key"], body)
			},
			render: autoRender("Schedule updated"),
		},
		{
			Key: "user.update", DisplayName: "Update user", Verb: "update", Resource: "user.update",
			ArgSchema: []ArgSpec{
				{Name: "id", Required: true, Type: "int"},
				{Name: "role", Required: false, Type: "string", Choices: []string{"user", "admin", "super_admin"}},
				{Name: "full_name", Required: false, Type: "string"},
				{Name: "email", Required: false, Type: "string"},
				{Name: "phone", Required: false, Type: "string"},
			},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				body := map[string]any{}
				for _, k := range []string{"role", "full_name", "email", "phone"} {
					if a[k] != "" {
						body[k] = a[k]
					}
				}
				return putJSON(ctx, c, jwt, "/users/"+a["id"], body)
			},
			render: autoRender("User updated"),
		},

		// ---- DELETE ----
		{
			Key: "backup.delete", DisplayName: "Delete backup", Verb: "delete", Resource: "backup.delete",
			ArgSchema: []ArgSpec{{Name: "filename", Required: true, Type: "string"}},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return deleteJSON(ctx, c, jwt, "/backups/"+a["filename"])
			},
			render: autoRender("Backup deleted"),
		},
		{
			Key: "user.delete", DisplayName: "Delete user", Verb: "delete", Resource: "user.delete",
			ArgSchema: []ArgSpec{{Name: "id", Required: true, Type: "int"}},
			execute: func(ctx context.Context, c client.HTTPClient, jwt string, a map[string]string) (any, int, error) {
				return deleteJSON(ctx, c, jwt, "/users/"+a["id"])
			},
			render: autoRender("User deleted"),
		},
	}
}
