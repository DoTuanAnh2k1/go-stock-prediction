package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"golang.org/x/crypto/bcrypt"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"` // "admin" or "user"
}

type userResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// ListUsersHandler handles GET /api/users — admin only
func ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	store := repository.GetSingleton()
	users, err := store.GetAllUsers()
	if err != nil {
		ResponseError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}
	resp := make([]userResponse, len(users))
	for i, u := range users {
		resp[i] = userResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	ResponseSuccess(w, http.StatusOK, resp)
}

// CreateUserHandler handles POST /api/users — admin only
func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		ResponseError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Logger.Errorf("bcrypt error: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	store := repository.GetSingleton()
	user := &modelsdb.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
	}
	if err := store.CreateUser(user); err != nil {
		ResponseError(w, http.StatusConflict, "username already exists")
		return
	}
	ResponseSuccess(w, http.StatusCreated, userResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// DeleteUserHandler handles DELETE /api/users/{id} — admin only
func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	// Prevent self-deletion
	claims := getClaims(r)
	var callerID uint64
	if uid, ok := claims["user_id"].(float64); ok {
		callerID = uint64(uid)
	}
	if callerID == id {
		ResponseError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}
	store := repository.GetSingleton()
	if err := store.DeleteUser(uint(id)); err != nil {
		ResponseError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user deleted"})
}
