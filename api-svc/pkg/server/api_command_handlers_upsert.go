package server

import (
	"encoding/json"
	"net/http"

	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
)

// upsertHandlersRequest is the inbound JSON body for the internal handler-catalog
// upsert endpoint. `secret` is validated against config INTERNAL_SECRET; the
// remaining fields describe the CLI handler catalog supplied by cli-svc.
type upsertHandlersRequest struct {
	Secret   string `json:"secret"`
	Handlers []struct {
		HandlerKey  string          `json:"handler_key"`
		DisplayName string          `json:"display_name"`
		Verb        string          `json:"verb"`
		Resource    string          `json:"resource"`
		ArgSchema   json.RawMessage `json:"arg_schema"`
		Enabled     bool            `json:"enabled"`
	} `json:"handlers"`
}

// UpsertCommandHandlersHandler godoc
//
//	@Summary      Upsert CLI handler catalog (internal)
//	@Description  Internal endpoint used by cli-svc to push its handler catalog. No JWT — protected by a shared secret in the request body (config INTERNAL_SECRET).
//	@Tags         Commands
//	@Accept       json
//	@Produce      json
//	@Param        body body upsertHandlersRequest true "Handler catalog + shared secret"
//	@Success      200 {object} map[string]string
//	@Failure      401 {object} ResponseFailure
//	@Router       /api/command-handlers/upsert [post]
func UpsertCommandHandlersHandler(w http.ResponseWriter, r *http.Request) {
	var req upsertHandlersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	secret := config.GetServerConfig().InternalSecret
	// When no secret is configured, reject to avoid an accidentally open endpoint.
	if secret == "" || req.Secret != secret {
		ResponseError(w, http.StatusUnauthorized, "invalid internal secret")
		return
	}

	client := requireAuthClient(w)
	if client == nil {
		return
	}

	handlers := make([]*authpb.CliHandler, 0, len(req.Handlers))
	for _, h := range req.Handlers {
		argSchema := "[]"
		if len(h.ArgSchema) > 0 {
			argSchema = string(h.ArgSchema)
		}
		handlers = append(handlers, &authpb.CliHandler{
			HandlerKey:  h.HandlerKey,
			DisplayName: h.DisplayName,
			Verb:        h.Verb,
			Resource:    h.Resource,
			ArgSchema:   argSchema,
			Enabled:     h.Enabled,
		})
	}

	_, err := client.UpsertHandlers(r.Context(), &authpb.UpsertHandlersRequest{
		Secret:   req.Secret,
		Handlers: handlers,
	})
	if err != nil {
		logger.Logger.Errorf("authclient.UpsertHandlers: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "handlers upserted"})
}
