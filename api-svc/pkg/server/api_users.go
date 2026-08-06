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

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// ListUsersHandler godoc
//
//	@Summary      List users
//	@Description  Returns all non-deleted users; admin or super_admin required
//	@Tags         Users
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array} authpb.UserResponse
//	@Failure      401 {object} ResponseFailure
//	@Failure      403 {object} ResponseFailure
//	@Router       /api/users [get]
func ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	resp, err := client.ListUsers(r.Context(), callerFromClaims(getClaims(r)))
	if err != nil {
		logger.Ctx(r.Context()).Errorf("authclient.ListUsers error: %v", err)
		ResponseError(w, http.StatusInternalServerError, "auth service error")
		return
	}
	ResponseSuccess(w, http.StatusOK, resp.Users)
}

// CreateUserHandler godoc
//
//	@Summary      Create user
//	@Description  Creates a new user; admin or super_admin required
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body createUserRequest true "User details"
//	@Success      201 {object} authpb.UserResponse
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      403 {object} ResponseFailure
//	@Failure      409 {object} ResponseFailure
//	@Router       /api/users [post]
func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	resp, err := client.CreateUser(r.Context(), &authpb.CreateUserRequest{
		Caller:   callerFromClaims(getClaims(r)),
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.AlreadyExists:
			ResponseError(w, http.StatusConflict, "username already taken")
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		default:
			logger.Ctx(r.Context()).Errorf("authclient.CreateUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusCreated, resp)
}

// DeleteUserHandler godoc
//
//	@Summary      Delete user
//	@Description  Soft-deletes a user by ID; cannot delete super_admin
//	@Tags         Users
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "User ID"
//	@Success      200 {object} map[string]string
//	@Failure      400 {object} ResponseFailure
//	@Failure      401 {object} ResponseFailure
//	@Failure      403 {object} ResponseFailure
//	@Failure      404 {object} ResponseFailure
//	@Router       /api/users/{id} [delete]
func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	caller := callerFromClaims(getClaims(r))
	if caller.CallerId == targetID {
		ResponseError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	_, err = client.DeleteUser(r.Context(), &authpb.DeleteUserRequest{
		Caller: caller, TargetId: targetID,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.NotFound:
			ResponseError(w, http.StatusNotFound, "user not found")
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		default:
			logger.Ctx(r.Context()).Errorf("authclient.DeleteUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

type updateUserRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
}

// UpdateUserHandler godoc
//
//	@Summary      Update user
//	@Description  Updates profile fields (full_name, email, phone) and/or role of an existing user; admin or super_admin required
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path     int               true  "User ID"
//	@Param        body  body     updateUserRequest true  "Fields to update"
//	@Success      200   {object} authpb.UserResponse
//	@Failure      400   {object} ResponseFailure
//	@Failure      401   {object} ResponseFailure
//	@Failure      403   {object} ResponseFailure
//	@Failure      404   {object} ResponseFailure
//	@Failure      503   {object} ResponseFailure
//	@Router       /api/users/{id} [put]
func UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	resp, err := client.UpdateUser(r.Context(), &authpb.UpdateUserRequest{
		Caller:   callerFromClaims(getClaims(r)),
		TargetId: targetID,
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Role:     req.Role,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		case codes.NotFound:
			ResponseError(w, http.StatusNotFound, "user not found")
		case codes.InvalidArgument:
			ResponseError(w, http.StatusBadRequest, st.Message())
		default:
			logger.Ctx(r.Context()).Errorf("authclient.UpdateUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusOK, resp)
}

// ResetPasswordHandler godoc
//
//	@Summary      Reset user password
//	@Description  Resets the password of an existing user; admin or super_admin required; cannot reset super_admin password if caller is admin
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path     int                  true "User ID"
//	@Param        body  body     resetPasswordRequest true "New password"
//	@Success      200   {object} map[string]string
//	@Failure      400   {object} ResponseFailure
//	@Failure      401   {object} ResponseFailure
//	@Failure      403   {object} ResponseFailure
//	@Failure      404   {object} ResponseFailure
//	@Failure      503   {object} ResponseFailure
//	@Router       /api/users/{id}/reset-password [post]
func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	_, err = client.ResetPassword(r.Context(), &authpb.ResetPasswordRequest{
		Caller:      callerFromClaims(getClaims(r)),
		TargetId:    targetID,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		case codes.NotFound:
			ResponseError(w, http.StatusNotFound, "user not found")
		case codes.InvalidArgument:
			ResponseError(w, http.StatusBadRequest, st.Message())
		default:
			logger.Ctx(r.Context()).Errorf("authclient.ResetPassword error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "password reset"})
}
