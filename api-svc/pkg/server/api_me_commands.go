package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
)

// GetMyCommandsHandler godoc
//
//	@Summary      List commands the current user may run
//	@Description  Returns the commands allowed for the authenticated caller (union over their command groups; super_admin/admin get all enabled commands).
//	@Tags         Commands
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array} commandDTO
//	@Router       /api/me/commands [get]
func GetMyCommandsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}
	claims := getClaims(r)
	userIDFloat, _ := claims["user_id"].(float64)
	client := requireAuthClient(w)
	if client == nil {
		return
	}
	resp, err := client.GetUserCommands(r.Context(), &authpb.UserRequest{
		Caller: callerFromClaims(claims),
		UserId: int64(userIDFloat),
	})
	if err != nil {
		logger.Ctx(r.Context()).Errorf("authclient.GetUserCommands: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusOK, commandsToDTO(resp.Commands))
}
