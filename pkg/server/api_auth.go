package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
	"golang.org/x/crypto/bcrypt"
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
//	@Description  Authenticate with username and password, returns a JWT valid for 24 hours
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body body loginRequest true "Login credentials"
//	@Success      200 {object} loginResponse
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      500 {object} ResponseFailure
//	@Router       /api/auth/login [post]
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	store := repository.GetSingleton()
	user, err := store.GetUserByUsername(req.Username)
	if err != nil {
		ResponseError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		ResponseError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	cfg := config.GetServerConfig()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     user.Username,
		"role":    user.Role,
		"user_id": user.ID,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	signed, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		logger.Logger.Errorf("Failed to sign JWT: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	ResponseSuccess(w, http.StatusOK, loginResponse{
		Token: signed,
		User:  userInfo{Username: user.Username, Role: user.Role},
	})
}

// MeHandler godoc
//
//	@Summary      Get current user
//	@Description  Returns the authenticated user's username and role based on the JWT token
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
	username, _ := claims["sub"].(string)
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
//	@Description  Changes the authenticated user's password; requires the current password for verification
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body changePasswordRequest true "Current and new password"
//	@Success      200 {object} map[string]string
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      404 {object} ResponseFailure
//	@Failure      500 {object} ResponseFailure
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
	if len(req.NewPassword) < 6 {
		ResponseError(w, http.StatusBadRequest, "new password must be at least 6 characters")
		return
	}

	claims := getClaims(r)
	userIDFloat, _ := claims["user_id"].(float64)
	userID := uint(userIDFloat)

	store := repository.GetSingleton()
	user, err := store.GetUserByID(userID)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		ResponseError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Logger.Errorf("bcrypt error: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := store.UpdateUserPassword(userID, string(newHash)); err != nil {
		ResponseError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "password updated successfully"})
}
