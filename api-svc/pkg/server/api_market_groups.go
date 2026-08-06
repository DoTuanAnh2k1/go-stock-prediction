package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	authclient "go-stock-prediction/pkg/grpc/authclient"
	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type createGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type setGroupMarketsRequest struct {
	MarketKeys []string `json:"market_keys"`
}

type addUserToGroupRequest struct {
	UserID int64 `json:"user_id"`
}

func requireAuthClient(w http.ResponseWriter) authpb.AuthServiceClient {
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return nil
	}
	return client
}

// ListMarketGroupsHandler godoc
//
//	@Summary      List market groups
//	@Tags         MarketGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array} authpb.MarketGroupResponse
//	@Router       /api/market-groups [get]
func ListMarketGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.ListMarketGroups(r.Context(), callerFromClaims(getClaims(r)))
	if err != nil {
		logger.Ctx(r.Context()).Errorf("authclient.ListMarketGroups: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusOK, resp.Groups)
}

// CreateMarketGroupHandler godoc
//
//	@Summary      Create market group
//	@Tags         MarketGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body createGroupRequest true "Group details"
//	@Success      201 {object} authpb.MarketGroupResponse
//	@Router       /api/market-groups [post]
func CreateMarketGroupHandler(w http.ResponseWriter, r *http.Request) {
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
	resp, err := client.CreateMarketGroup(r.Context(), &authpb.CreateGroupRequest{
		Caller: callerFromClaims(getClaims(r)), Name: req.Name, Description: req.Description,
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			ResponseError(w, http.StatusConflict, "group name already exists")
			return
		}
		logger.Ctx(r.Context()).Errorf("authclient.CreateMarketGroup: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusCreated, resp)
}

// UpdateMarketGroupHandler godoc
//
//	@Summary      Update market group
//	@Tags         MarketGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        body body createGroupRequest true "New name/description"
//	@Success      200 {object} authpb.MarketGroupResponse
//	@Router       /api/market-groups/{id} [put]
func UpdateMarketGroupHandler(w http.ResponseWriter, r *http.Request) {
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
	resp, err := client.UpdateMarketGroup(r.Context(), &authpb.UpdateGroupRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID,
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, resp)
}

// DeleteMarketGroupHandler godoc
//
//	@Summary      Delete market group
//	@Tags         MarketGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/market-groups/{id} [delete]
func DeleteMarketGroupHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err = client.DeleteMarketGroup(r.Context(), &authpb.DeleteGroupRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "group deleted"})
}

// SetGroupMarketsHandler godoc
//
//	@Summary      Set markets for a group (replaces existing)
//	@Tags         MarketGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        body body setGroupMarketsRequest true "Market keys"
//	@Success      200 {object} map[string]string
//	@Router       /api/market-groups/{id}/markets [put]
func SetGroupMarketsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	groupID, err := parseGroupID(w, r)
	if err != nil {
		return
	}
	var req setGroupMarketsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	_, err = client.SetGroupMarkets(r.Context(), &authpb.SetGroupMarketsRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID, MarketKeys: req.MarketKeys,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "markets updated"})
}

// ListGroupUsersHandler godoc
//
//	@Summary      List users in a market group
//	@Tags         MarketGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Success      200 {array} authpb.UserResponse
//	@Router       /api/market-groups/{id}/users [get]
func ListGroupUsersHandler(w http.ResponseWriter, r *http.Request) {
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
	resp, err := client.ListGroupUsers(r.Context(), &authpb.GroupRequest{
		Caller: callerFromClaims(getClaims(r)), GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, resp.Users)
}

// AddUserToGroupHandler godoc
//
//	@Summary      Add user to market group
//	@Tags         MarketGroups
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        body body addUserToGroupRequest true "User ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/market-groups/{id}/users [post]
func AddUserToGroupHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err = client.AddUserToGroup(r.Context(), &authpb.UserGroupRequest{
		Caller: callerFromClaims(getClaims(r)), UserId: req.UserID, GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user added to group"})
}

// RemoveUserFromGroupHandler godoc
//
//	@Summary      Remove user from market group
//	@Tags         MarketGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "Group ID"
//	@Param        uid path int true "User ID"
//	@Success      200 {object} map[string]string
//	@Router       /api/market-groups/{id}/users/{uid} [delete]
func RemoveUserFromGroupHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err = client.RemoveUserFromGroup(r.Context(), &authpb.UserGroupRequest{
		Caller: callerFromClaims(getClaims(r)), UserId: userID, GroupId: groupID,
	})
	if err != nil {
		handleGroupError(w, err)
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user removed from group"})
}

// GetUserMarketGroupsHandler godoc
//
//	@Summary      Get market groups for a user
//	@Tags         MarketGroups
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "User ID"
//	@Success      200 {array} authpb.MarketGroupResponse
//	@Router       /api/users/{id}/market-groups [get]
func GetUserMarketGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}
	idStr := r.PathValue("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.GetUserMarketGroups(r.Context(), &authpb.UserRequest{
		Caller: callerFromClaims(getClaims(r)), UserId: userID,
	})
	if err != nil {
		logger.Ctx(r.Context()).Errorf("authclient.GetUserMarketGroups: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusOK, resp.Groups)
}

func parseGroupID(w http.ResponseWriter, r *http.Request) (int64, error) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid group id")
		return 0, err
	}
	return id, nil
}

func handleGroupError(w http.ResponseWriter, err error) {
	st, _ := status.FromError(err)
	switch st.Code() {
	case codes.NotFound:
		ResponseError(w, http.StatusNotFound, st.Message())
	case codes.AlreadyExists:
		ResponseError(w, http.StatusConflict, st.Message())
	case codes.PermissionDenied:
		ResponseError(w, http.StatusForbidden, st.Message())
	case codes.InvalidArgument:
		ResponseError(w, http.StatusBadRequest, st.Message())
	default:
		logger.Logger.Errorf("authclient market group error: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
	}
}
