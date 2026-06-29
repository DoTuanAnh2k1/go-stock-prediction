package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	authclient "go-stock-prediction/pkg/grpc/authclient"
	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grantTokenHeader carries base64("username:password"). Replaces the old
// username/password JSON body so the login surface is less obvious to
// automated scanners.
const grantTokenHeader = "X-Token"

type loginResponse struct {
	Token string   `json:"token"`
	User  userInfo `json:"user"`
}

type userInfo struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// decodeGrantToken decodes the X-Token header (base64 of "username:password")
// and splits on the first ':' so passwords containing ':' survive. Returns
// ok=false when the header is absent, not valid base64, or has no separator.
func decodeGrantToken(header string) (username, password string, ok bool) {
	if header == "" {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header))
	if err != nil {
		return "", "", false
	}
	user, pass, found := strings.Cut(string(raw), ":")
	if !found {
		return "", "", false
	}
	return user, pass, true
}

// LoginHandler godoc
//
//	@Summary      Login
//	@Description  Authenticate via the X-Token header (base64 of "username:password"); returns JWT signed by Java Auth Service. Body is ignored.
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        X-Token header string true "base64(\"username:password\")"
//	@Success      200 {object} loginResponse
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      503 {object} ResponseFailure
//	@Router       /api/x/grant [post]
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	username, password, ok := decodeGrantToken(r.Header.Get(grantTokenHeader))
	if !ok {
		ResponseError(w, http.StatusBadRequest, "invalid request")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	resp, err := client.Login(r.Context(), &authpb.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.Unauthenticated:
			ResponseError(w, http.StatusUnauthorized, "invalid credentials")
		default:
			logger.Logger.Errorf("authclient.Login error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusOK, loginResponse{
		Token: resp.Token,
		User:  userInfo{Username: resp.Username, Role: resp.Role},
	})
}

// MeHandler godoc
//
//	@Summary      Get current user
//	@Description  Returns username and role from JWT claims (no DB call)
//	@Tags         Auth
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {object} userInfo
//	@Failure      401 {object} ResponseFailure
//	@Router       /api/auth/me [get]
func MeHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}
	claims := getClaims(r)
	username, _ := claims["username"].(string)
	if username == "" {
		// fallback: some tokens use "sub" for username
		username, _ = claims["sub"].(string)
	}
	role, _ := claims["role"].(string)
	ResponseSuccess(w, http.StatusOK, userInfo{Username: username, Role: role})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePasswordHandler godoc
//
//	@Summary      Change password
//	@Description  Proxies to Java Auth Service — validates current password before updating
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body changePasswordRequest true "Current and new password"
//	@Success      200 {object} map[string]string
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      503 {object} ResponseFailure
//	@Router       /api/auth/password [put]
func ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	claims := getClaims(r)
	_, err := client.ChangePassword(r.Context(), &authpb.ChangePassRequest{
		Caller:      callerFromClaims(claims),
		OldPassword: req.CurrentPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.Unauthenticated:
			ResponseError(w, http.StatusUnauthorized, "current password is incorrect")
		case codes.InvalidArgument:
			ResponseError(w, http.StatusBadRequest, st.Message())
		default:
			logger.Logger.Errorf("authclient.ChangePassword error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "password updated successfully"})
}
