package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// commandGroupDTO is the JSON shape returned for a command group.
type commandGroupDTO struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CommandIDs  []int64 `json:"command_ids"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type setGroupCommandsRequest struct {
	CommandIDs []int64 `json:"command_ids"`
}

func commandGroupToDTO(g *authpb.CommandGroupResponse) commandGroupDTO {
	ids := g.GetCommandIds()
	if ids == nil {
		ids = []int64{}
	}
	return commandGroupDTO{
		ID:          g.GetId(),
		Name:        g.GetName(),
		Description: g.GetDescription(),
		CommandIDs:  ids,
		CreatedAt:   g.GetCreatedAt(),
		UpdatedAt:   g.GetUpdatedAt(),
	}
}

// ListCommandGroupsHandler godoc
//
//	@Summary      List command groups
//	@Tags         CommandGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array} commandGroupDTO
//	@Router       /api/command-groups [get]
func ListCommandGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.ListCommandGroups(r.Context(), callerFromClaims(getClaims(r)))
	if err != nil {
		logger.Logger.Errorf("authclient.ListCommandGroups: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	out := make([]commandGroupDTO, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		out = append(out, commandGroupToDTO(g))
	}
	ResponseSuccess(w, http.StatusOK, out)
}

// CreateCommandGroupHandler godoc
//
//	@Summary      Create command group
//	@Tags         CommandGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body createGroupRequest true "Group details"
//	@Success      201 {object} commandGroupDTO
//	@Router       /api/command-groups [post]
func CreateCommandGroupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.CreateCommandGroup(r.Context(), &authpb.CreateCmdGroupRequest{
		Caller: callerFromClaims(getClaims(r)), Name: req.Name, Description: req.Description,
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			ResponseError(w, http.StatusConflict, "group name already exists")
			return
		}
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusCreated, commandGroupToDTO(resp))
}

// UpdateCommandGroupHandler godoc
//
//	@Summary      Update command group
//	@Tags         CommandGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        body body createGroupRequest true "New name/description"
//	@Success      200 {object} commandGroupDTO
//	@Router       /api/command-groups/{id} [put]
func UpdateCommandGroupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.UpdateCommandGroup(r.Context(), &authpb.UpdateCmdGroupRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID,
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, commandGroupToDTO(resp))
}

// DeleteCommandGroupHandler godoc
//
//	@Summary      Delete command group
//	@Tags         CommandGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/command-groups/{id} [delete]
func DeleteCommandGroupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	_, err = client.DeleteCommandGroup(r.Context(), &authpb.DeleteCmdGroupRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "group deleted"})
}

// SetGroupCommandsHandler godoc
//
//	@Summary      Set commands for a group (replaces existing)
//	@Tags         CommandGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        body body setGroupCommandsRequest true "Command IDs"
//	@Success      200 {object} map[string]string
//	@Router       /api/command-groups/{id}/commands [put]
func SetGroupCommandsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	var req setGroupCommandsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	_, err = client.SetGroupCommands(r.Context(), &authpb.SetGroupCommandsRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID, CommandIds: req.CommandIDs,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "commands updated"})
}

// ListCmdGroupUsersHandler godoc
//
//	@Summary      List users in a command group
//	@Tags         CommandGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Success      200 {array} authpb.UserResponse
//	@Router       /api/command-groups/{id}/users [get]
func ListCmdGroupUsersHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.ListCmdGroupUsers(r.Context(), &authpb.CmdGroupRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, resp.Users)
}

// AddUserToCmdGroupHandler godoc
//
//	@Summary      Add user to command group
//	@Tags         CommandGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        body body addUserToGroupRequest true "User ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/command-groups/{id}/users [post]
func AddUserToCmdGroupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	var req addUserToGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	_, err = client.AddUserToCmdGroup(r.Context(), &authpb.UserCmdGroupRequest{
		Caller: callerFromClaims(getClaims(r)), UserId: req.UserID, GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user added to group"})
}

// RemoveUserFromCmdGroupHandler godoc
//
//	@Summary      Remove user from command group
//	@Tags         CommandGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        uid path int true "User ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/command-groups/{id}/users/{uid} [delete]
func RemoveUserFromCmdGroupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	uidStr := r.PathValue("uid")
	userID, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	_, err = client.RemoveUserFromCmdGroup(r.Context(), &authpb.UserCmdGroupRequest{
		Caller: callerFromClaims(getClaims(r)), UserId: userID, GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user removed from group"})
}
