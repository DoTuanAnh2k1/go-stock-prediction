package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
)

// commandDTO is the JSON shape returned for a command. The proto carries `args`
// as a JSON-object string; at the HTTP boundary we expose it as a JSON object.
type commandDTO struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	HandlerKey  string          `json:"handler_key"`
	Args        json.RawMessage `json:"args"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// handlerDTO is the JSON shape returned for a CLI handler catalog entry. The
// proto carries `arg_schema` as a JSON-array string; we expose it as a JSON array.
type handlerDTO struct {
	HandlerKey  string          `json:"handler_key"`
	DisplayName string          `json:"display_name"`
	Verb        string          `json:"verb"`
	Resource    string          `json:"resource"`
	ArgSchema   json.RawMessage `json:"arg_schema"`
	Enabled     bool            `json:"enabled"`
}

// createCommandRequest is the inbound JSON body for create/update command.
type createCommandRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	HandlerKey  string          `json:"handler_key"`
	Args        json.RawMessage `json:"args"`
	Enabled     bool            `json:"enabled"`
}

// rawOrDefault returns the given JSON string as RawMessage, falling back to the
// supplied default when empty/blank.
func rawOrDefault(s, def string) json.RawMessage {
	if s == "" {
		return json.RawMessage(def)
	}
	return json.RawMessage(s)
}

// argsToString marshals an inbound JSON object/array into a compact string for
// the proto boundary, defaulting to def when nil/empty.
func argsToString(raw json.RawMessage, def string) string {
	if len(raw) == 0 {
		return def
	}
	return string(raw)
}

func commandToDTO(c *authpb.Command) commandDTO {
	return commandDTO{
		ID:          c.GetId(),
		Name:        c.GetName(),
		Description: c.GetDescription(),
		HandlerKey:  c.GetHandlerKey(),
		Args:        rawOrDefault(c.GetArgs(), "{}"),
		Enabled:     c.GetEnabled(),
		CreatedAt:   c.GetCreatedAt(),
		UpdatedAt:   c.GetUpdatedAt(),
	}
}

func commandsToDTO(cmds []*authpb.Command) []commandDTO {
	out := make([]commandDTO, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, commandToDTO(c))
	}
	return out
}

func handlerToDTO(h *authpb.CliHandler) handlerDTO {
	return handlerDTO{
		HandlerKey:  h.GetHandlerKey(),
		DisplayName: h.GetDisplayName(),
		Verb:        h.GetVerb(),
		Resource:    h.GetResource(),
		ArgSchema:   rawOrDefault(h.GetArgSchema(), "[]"),
		Enabled:     h.GetEnabled(),
	}
}

// ListCommandsHandler godoc
//
//	@Summary      List commands
//	@Tags         Commands
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array} commandDTO
//	@Router       /api/commands [get]
func ListCommandsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.ListCommands(r.Context(), callerFromClaims(getClaims(r)))
	if err != nil {
		logger.Logger.Errorf("authclient.ListCommands: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusOK, commandsToDTO(resp.Commands))
}

// CreateCommandHandler godoc
//
//	@Summary      Create command
//	@Tags         Commands
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body createCommandRequest true "Command details"
//	@Success      201 {object} commandDTO
//	@Router       /api/commands [post]
func CreateCommandHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req createCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.CreateCommand(r.Context(), &authpb.CreateCommandRequest{
		Caller:      callerFromClaims(getClaims(r)),
		Name:        req.Name,
		Description: req.Description,
		HandlerKey:  req.HandlerKey,
		Args:        argsToString(req.Args, "{}"),
		Enabled:     req.Enabled,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusCreated, commandToDTO(resp.GetCommand()))
}

// UpdateCommandHandler godoc
//
//	@Summary      Update command
//	@Tags         Commands
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Command ID"
//	@Param        body body createCommandRequest true "Command details"
//	@Success      200 {object} commandDTO
//	@Router       /api/commands/{id} [put]
func UpdateCommandHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := parseCommandID(w, r)
	if err != nil {
		return
	}
	var req createCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.UpdateCommand(r.Context(), &authpb.UpdateCommandRequest{
		Caller:      callerFromClaims(getClaims(r)),
		CommandId:   id,
		Name:        req.Name,
		Description: req.Description,
		HandlerKey:  req.HandlerKey,
		Args:        argsToString(req.Args, "{}"),
		Enabled:     req.Enabled,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, commandToDTO(resp.GetCommand()))
}

// DeleteCommandHandler godoc
//
//	@Summary      Delete command
//	@Tags         Commands
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Command ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/commands/{id} [delete]
func DeleteCommandHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := parseCommandID(w, r)
	if err != nil {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	_, err = client.DeleteCommand(r.Context(), &authpb.DeleteCommandRequest{
		Caller: callerFromClaims(getClaims(r)), CommandId: id,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "command deleted"})
}

// ListCommandHandlersHandler godoc
//
//	@Summary      List CLI handler catalog
//	@Tags         Commands
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array} handlerDTO
//	@Router       /api/command-handlers [get]
func ListCommandHandlersHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.ListHandlers(r.Context(), callerFromClaims(getClaims(r)))
	if err != nil {
		logger.Logger.Errorf("authclient.ListHandlers: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	out := make([]handlerDTO, 0, len(resp.Handlers))
	for _, h := range resp.Handlers {
		out = append(out, handlerToDTO(h))
	}
	ResponseSuccess(w, http.StatusOK, out)
}

func parseCommandID(w http.ResponseWriter, r *http.Request) (int64, error) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid command id")
		return 0, err
	}
	return id, nil
}
