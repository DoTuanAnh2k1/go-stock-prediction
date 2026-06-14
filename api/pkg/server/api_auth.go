package server

import (
	"encoding/json"
	"net/http"

	authclient "go-stock-prediction/pkg/grpc/authclient"
	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string   `json:"token"`
	User  userInfo `json:"user"`
}

type userInfo struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// LoginHandler godoc
//
//	@Summary      Login
//	@Description  Authenticate with username/password; returns JWT signed by Java Auth Service
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body body loginRequest true "Login credentials"
//	@Success      200 {object} loginResponse
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      503 {object} ResponseFailure
//	@Router       /api/auth/login [post]
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	resp, err := client.Login(r.Context(), &authpb.LoginRequest{
		Username: req.Username,
		Password: req.Password,
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
