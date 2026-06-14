# Go API Auth Proxy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Chuyển Go API từ tự xử lý auth sang thin proxy — generate Go gRPC stubs từ auth.proto, xây dựng auth gRPC client, proxy tất cả user/auth/market-group endpoints sang Java Auth Service, cập nhật middleware với `MarketRequired`/`AdminRequired`, và áp dụng access control lên tất cả market endpoints.

**Architecture:** Go API giữ nguyên JWT validation local (đọc claims từ token do Java ký). Mọi thao tác auth (login, change password, user CRUD, market group CRUD) đều gọi gRPC sang Java Auth Service `:8120`. Market endpoints require `AuthRequired + MarketRequired("KEY")`. Trigger endpoints require `AdminRequired`. Monitoring/simulation require auth và filter response theo `accessible_markets` trong JWT claims.

**Tech Stack:** Go 1.22+, google.golang.org/grpc, protoc + protoc-gen-go + protoc-gen-go-grpc (đã có trong môi trường từ prediction service proto).

**Prerequisite:** Plan 1 (Java Auth Service) đã complete và image build thành công. `api/proto/auth/auth.proto` đã tồn tại (copy từ Plan 1 Task 2).

---

## File Map

### Tạo mới
```
api/proto/auth/
├── auth.pb.go          ← generated
└── auth_grpc.pb.go     ← generated

api/pkg/grpc/authclient/
└── client.go

api/pkg/server/
└── api_market_groups.go
```

### Sửa đổi
```
api/pkg/models/models_config/config.go    ← thêm AuthGRPCConfig
api/pkg/config/init.go                    ← load AUTH_GRPC_TARGET
api/pkg/config/config.go                  ← thêm GetAuthGRPCConfig()
api/cmd/main.go                           ← bỏ seedAdminUser, init authclient
api/pkg/server/middleware_jwt.go          ← thêm MarketRequired, AdminRequired, getAccessibleMarkets
api/pkg/server/api_auth.go               ← proxy Login, ChangePassword; giữ MeHandler local
api/pkg/server/api_users.go              ← proxy ListUsers, CreateUser, DeleteUser
api/pkg/server/router.go                 ← wrap market routes, trigger routes, add market-group routes
api/pkg/models/models_db/migrations.go   ← xóa &User{} khỏi AllModels
```

### Xóa
```
api/pkg/store/repository/user.go         ← UserStore interface (Java owns users)
api/pkg/store/mysql/user.go              ← MySQL UserStore implementation
```

---

## Task 1: Generate Go proto stubs

**Files:**
- Modify: `api/proto/auth/auth.proto` (đã có từ Plan 1)
- Create: `api/proto/auth/auth.pb.go` (generated)
- Create: `api/proto/auth/auth_grpc.pb.go` (generated)

- [ ] **Kiểm tra protoc tools có sẵn chưa:**
```bash
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
```
Nếu thiếu, cài:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

- [ ] **Generate Go stubs từ auth.proto:**
```bash
cd api
protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/auth/auth.proto
```
Expected: tạo ra `api/proto/auth/auth.pb.go` và `api/proto/auth/auth_grpc.pb.go`.

- [ ] **Kiểm tra stubs compile được:**
```bash
cd api && go build ./proto/auth/...
```
Expected: không có lỗi.

- [ ] **Commit:**
```bash
git add api/proto/auth/
git commit -m "feat(auth): generate Go gRPC stubs from auth.proto"
```

---

## Task 2: Auth gRPC client

**Files:**
- Create: `api/pkg/grpc/authclient/client.go`

- [ ] **Tạo `api/pkg/grpc/authclient/client.go`:**

```go
package authclient

import (
	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	conn       *grpc.ClientConn
	authClient authpb.AuthServiceClient
)

// Init creates a gRPC connection to the Java Auth Service and initialises the
// singleton AuthServiceClient. Calls logger.Logger.Fatalf on connection error.
func Init(target string) {
	logger.Logger.Infof("authclient: connecting to auth service at %s", target)
	c, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Logger.Fatalf("authclient: failed to connect to %s: %v", target, err)
		return
	}
	conn = c
	authClient = authpb.NewAuthServiceClient(conn)
	logger.Logger.Infof("authclient: connected to auth service at %s", target)
}

// GetClient returns the singleton AuthServiceClient. Init must be called first.
func GetClient() authpb.AuthServiceClient {
	return authClient
}

// Close closes the underlying gRPC connection (call on graceful shutdown).
func Close() {
	if conn != nil {
		if err := conn.Close(); err != nil {
			logger.Logger.Errorf("authclient: error closing connection: %v", err)
			return
		}
		logger.Logger.Infof("authclient: connection closed")
	}
}
```

- [ ] **Kiểm tra compile:**
```bash
cd api && go build ./pkg/grpc/authclient/...
```

- [ ] **Commit:**
```bash
git add api/pkg/grpc/authclient/
git commit -m "feat(auth): add authclient gRPC singleton for Java Auth Service"
```

---

## Task 3: Config — thêm AuthGRPCTarget

**Files:**
- Modify: `api/pkg/models/models_config/config.go`
- Modify: `api/pkg/config/init.go`
- Modify: `api/pkg/config/config.go`

- [ ] **Sửa `api/pkg/models/models_config/config.go` — thêm `AuthGRPCTarget` vào `GRPCConfig`:**

```go
type GRPCConfig struct {
	// ServerPort is the port the prediction gRPC server listens on
	ServerPort string
	// ClientTarget is the address the API backend uses to connect to the prediction gRPC server
	ClientTarget string
	// AuthClientTarget is the address of the Java Auth Service gRPC server
	AuthClientTarget string
}
```

- [ ] **Sửa `api/pkg/config/init.go` — thêm `AUTH_GRPC_TARGET` vào GRPC block:**

```go
GRPC: models_config.GRPCConfig{
    ServerPort:       env.GetEnv("GRPC_SERVER_PORT", "8119"),
    ClientTarget:     env.GetEnv("GRPC_TARGET", "localhost:8119"),
    AuthClientTarget: env.GetEnv("AUTH_GRPC_TARGET", "localhost:8120"),
},
```

- [ ] **Sửa `api/pkg/config/config.go` — thêm helper:**

```go
func GetAuthGRPCConfig() string {
	if config == nil {
		return "localhost:8120"
	}
	return config.GRPC.AuthClientTarget
}
```

- [ ] **Kiểm tra compile:**
```bash
cd api && go build ./pkg/config/... ./pkg/models/...
```

- [ ] **Commit:**
```bash
git add api/pkg/models/models_config/config.go api/pkg/config/init.go api/pkg/config/config.go
git commit -m "feat(auth): add AUTH_GRPC_TARGET config for Java Auth Service"
```

---

## Task 4: Cập nhật cmd/main.go

**Files:**
- Modify: `api/cmd/main.go`

- [ ] **Sửa `api/cmd/main.go` — xóa seedAdminUser, thêm authclient.Init:**

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"go-stock-prediction/pkg/config"
	authclient "go-stock-prediction/pkg/grpc/authclient"
	grpcclient "go-stock-prediction/pkg/grpc/client"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/server"
	"go-stock-prediction/pkg/store/repository"

	_ "go-stock-prediction/docs"
)

//	@title			Go Stock Prediction API
//	@version		1.0
//	@description	Vietnamese stock market prediction system — ML algorithms (VWMA, EMA, LSTM, ARIMA-GARCH, Ensemble) for VN30 stocks, gold, NASDAQ, crypto, and S&P 500.
//	@host			localhost:8118
//	@BasePath		/
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT token from POST /api/auth/login. Format: Bearer {token}

func main() {
	config.InitConfig()

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic(fmt.Sprintf("Failed to load timezone: %v", err))
	}
	time.Local = loc

	logger.Init()

	repository.Init()

	// Connect to Java Auth Service (gRPC :8120)
	authclient.Init(config.GetAuthGRPCConfig())

	// Connect to Python Prediction Service (gRPC :8119)
	grpcclient.Init(config.GetGRPCConfig().ClientTarget)

	go server.StartHTTPServer()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	sig := <-signals
	logger.Logger.Infof("Received signal %v — shutting down API service", sig)
	authclient.Close()
	grpcclient.Close()
	logger.Logger.Info("API service stopped")
}
```

- [ ] **Kiểm tra compile:**
```bash
cd api && go build ./cmd/...
```

- [ ] **Commit:**
```bash
git add api/cmd/main.go
git commit -m "feat(auth): init authclient on startup, remove seedAdminUser"
```

---

## Task 5: Cập nhật middleware_jwt.go

**Files:**
- Modify: `api/pkg/server/middleware_jwt.go`

- [ ] **Thay thế toàn bộ `api/pkg/server/middleware_jwt.go`:**

```go
package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go-stock-prediction/pkg/config"
)

type contextKey string

const claimsKey contextKey = "jwt_claims"

// JWTMiddleware parses the Bearer token and injects claims into context.
// Non-blocking — requests without a valid token continue as unauthenticated.
func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			cfg := config.GetServerConfig()
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWTSecret), nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					r = r.WithContext(context.WithValue(r.Context(), claimsKey, claims))
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// getClaims extracts JWT claims from context (nil if unauthenticated).
func getClaims(r *http.Request) jwt.MapClaims {
	claims, _ := r.Context().Value(claimsKey).(jwt.MapClaims)
	return claims
}

// getAccessibleMarkets extracts the accessible_markets list from JWT claims.
// Returns nil if claim is absent or malformed.
func getAccessibleMarkets(claims jwt.MapClaims) []string {
	raw, ok := claims["accessible_markets"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	markets := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			markets = append(markets, s)
		}
	}
	return markets
}

// isSuperAdmin returns true if the JWT role claim is "super_admin".
func isSuperAdmin(claims jwt.MapClaims) bool {
	role, _ := claims["role"].(string)
	return role == "super_admin"
}

// requireAuth returns false and writes 401 if no valid JWT claims in context.
func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if getClaims(r) == nil {
		ResponseError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	return true
}

// AuthRequired wraps a handler requiring JWT authentication.
func AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		next(w, r)
	}
}

// requireAdmin returns false and writes 403 if user is not admin or super_admin.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := getClaims(r)
	if claims == nil {
		ResponseError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	role, _ := claims["role"].(string)
	if role != "admin" && role != "super_admin" {
		ResponseError(w, http.StatusForbidden, "admin access required")
		return false
	}
	return true
}

// AdminRequired wraps a handler requiring admin or super_admin role.
func AdminRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		next(w, r)
	}
}

// callerFromClaims extracts caller_id and caller_role from JWT claims into a CallerMeta proto.
// Import authpb "go-stock-prediction/proto/auth" at top of file.
func callerFromClaims(claims jwt.MapClaims) *authpb.CallerMeta {
	idFloat, _ := claims["user_id"].(float64)
	role, _ := claims["role"].(string)
	return &authpb.CallerMeta{CallerId: int64(idFloat), CallerRole: role}
}

// MarketRequired wraps a handler requiring the caller to have access to a specific market.
// super_admin bypasses the check. admin/user must have the market key in their accessible_markets JWT claim.
func MarketRequired(marketKey string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims := getClaims(r)
			if claims == nil {
				ResponseError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if isSuperAdmin(claims) {
				next(w, r)
				return
			}
			for _, m := range getAccessibleMarkets(claims) {
				if m == marketKey {
					next(w, r)
					return
				}
			}
			ResponseError(w, http.StatusForbidden, "no access to market: "+marketKey)
		}
	}
}
```

- [ ] **Kiểm tra compile:**
```bash
cd api && go build ./pkg/server/...
```

- [ ] **Commit:**
```bash
git add api/pkg/server/middleware_jwt.go
git commit -m "feat(auth): add MarketRequired, AdminRequired, getAccessibleMarkets to JWT middleware"
```

---

## Task 6: api_auth.go — thin proxy

**Files:**
- Modify: `api/pkg/server/api_auth.go`

- [ ] **Thay thế toàn bộ `api/pkg/server/api_auth.go`:**

```go
package server

import (
	"encoding/json"
	"net/http"

	"go-stock-prediction/pkg/logger"
	authclient "go-stock-prediction/pkg/grpc/authclient"
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
//	@Description  Authenticate with username and password; returns JWT signed by Java Auth Service
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
//	@Description  Returns username and role from the JWT claims (no DB call)
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

	claims := getClaims(r)
	callerIDFloat, _ := claims["user_id"].(float64)
	callerID := int64(callerIDFloat)
	callerRole, _ := claims["role"].(string)

	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	_, err := client.ChangePassword(r.Context(), &authpb.ChangePassRequest{
		Caller: &authpb.CallerMeta{CallerId: callerID, CallerRole: callerRole},
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
```

- [ ] **Commit:**
```bash
git add api/pkg/server/api_auth.go
git commit -m "feat(auth): api_auth.go — proxy Login and ChangePassword to Java, MeHandler reads JWT locally"
```

---

## Task 7: api_users.go — thin proxy

**Files:**
- Modify: `api/pkg/server/api_users.go`

- [ ] **Thay thế toàn bộ `api/pkg/server/api_users.go`:**

```go
package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	authclient "go-stock-prediction/pkg/grpc/authclient"
	authpb "go-stock-prediction/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
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
	claims := getClaims(r)
	callerID, callerRole := extractCaller(claims)

	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	resp, err := client.ListUsers(r.Context(),
		&authpb.CallerMeta{CallerId: callerID, CallerRole: callerRole})
	if err != nil {
		logger.Logger.Errorf("authclient.ListUsers error: %v", err)
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

	claims := getClaims(r)
	callerID, callerRole := extractCaller(claims)

	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	resp, err := client.CreateUser(r.Context(), &authpb.CreateUserRequest{
		Caller:   &authpb.CallerMeta{CallerId: callerID, CallerRole: callerRole},
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.AlreadyExists:
			ResponseError(w, http.StatusConflict, "username already taken")
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		default:
			logger.Logger.Errorf("authclient.CreateUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}

	ResponseSuccess(w, http.StatusCreated, resp)
}

// DeleteUserHandler godoc
//
//	@Summary      Delete user
//	@Description  Soft-deletes a user by ID; admin or super_admin required; cannot delete super_admin
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

	claims := getClaims(r)
	callerID, callerRole := extractCaller(claims)

	if callerID == targetID {
		ResponseError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	client := authclient.GetClient()
	if client == nil {
		ResponseError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	_, err = client.DeleteUser(r.Context(), &authpb.DeleteUserRequest{
		Caller:   &authpb.CallerMeta{CallerId: callerID, CallerRole: callerRole},
		TargetId: targetID,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.NotFound:
			ResponseError(w, http.StatusNotFound, "user not found")
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		default:
			logger.Logger.Errorf("authclient.DeleteUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}

	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

// extractCaller reads caller_id and caller_role from JWT claims.
func extractCaller(claims interface{ get(string) interface{} }) (int64, string) {
	return 0, ""
}
```

Wait, the helper `extractCaller` needs to work with `jwt.MapClaims`. Let me fix that properly:

```go
package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	authclient "go-stock-prediction/pkg/grpc/authclient"
	authpb "go-stock-prediction/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang-jwt/jwt/v5"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// callerFromClaims extracts caller_id and caller_role from JWT claims.
func callerFromClaims(claims jwt.MapClaims) *authpb.CallerMeta {
	idFloat, _ := claims["user_id"].(float64)
	role, _ := claims["role"].(string)
	return &authpb.CallerMeta{CallerId: int64(idFloat), CallerRole: role}
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
		logger.Logger.Errorf("authclient.ListUsers error: %v", err)
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
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.AlreadyExists:
			ResponseError(w, http.StatusConflict, "username already taken")
		case codes.PermissionDenied:
			ResponseError(w, http.StatusForbidden, st.Message())
		default:
			logger.Logger.Errorf("authclient.CreateUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusCreated, resp)
}

// DeleteUserHandler godoc
//
//	@Summary      Delete user
//	@Description  Soft-deletes a user by ID; cannot delete super_admin (unless caller is super_admin)
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
	claims := getClaims(r)
	caller := callerFromClaims(claims)
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
			logger.Logger.Errorf("authclient.DeleteUser error: %v", err)
			ResponseError(w, http.StatusInternalServerError, "auth service error")
		}
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "user deleted"})
}
```

- [ ] **Kiểm tra compile:**
```bash
cd api && go build ./pkg/server/...
```

- [ ] **Commit:**
```bash
git add api/pkg/server/api_users.go
git commit -m "feat(auth): api_users.go — proxy ListUsers, CreateUser, DeleteUser to Java"
```

---

## Task 8: api_market_groups.go — mới

**Files:**
- Create: `api/pkg/server/api_market_groups.go`

- [ ] **Tạo `api/pkg/server/api_market_groups.go`:**

```go
package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	authclient "go-stock-prediction/pkg/grpc/authclient"
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
//	@Description  Returns all market groups with their market keys; admin or super_admin required
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
		logger.Logger.Errorf("authclient.ListMarketGroups: %v", err)
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
//	@Param        body body createGroupRequest true "Group name and description"
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
		logger.Logger.Errorf("authclient.CreateMarketGroup: %v", err)
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
//	@Param        body body createGroupRequest true "New name and description"
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
		logger.Logger.Errorf("authclient.GetUserMarketGroups: %v", err)
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
```

- [ ] **Kiểm tra compile:**
```bash
cd api && go build ./pkg/server/...
```

- [ ] **Commit:**
```bash
git add api/pkg/server/api_market_groups.go
git commit -m "feat(auth): add api_market_groups.go — full CRUD proxy to Java"
```

---

## Task 9: Cập nhật router.go

**Files:**
- Modify: `api/pkg/server/router.go`

- [ ] **Thay thế toàn bộ `api/pkg/server/router.go`:**

```go
package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func addHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// Swagger UI
	mux.Handle("/swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	// Health
	mux.HandleFunc("/health", HealthCheckHandler)
	mux.HandleFunc("/health/simple", SimpleHealthHandler)
	mux.HandleFunc("/health/ready", ReadyHandler)

	// ── Auth (proxy → Java Auth Service) ────────────────────────────
	mux.HandleFunc("POST /api/auth/login", LoginRateLimitMiddleware(LoginHandler))
	mux.HandleFunc("GET /api/auth/me", AuthRequired(MeHandler))
	mux.HandleFunc("PUT /api/auth/password", AuthRequired(ChangePasswordHandler))

	// ── Dashboard ────────────────────────────────────────────────────
	mux.HandleFunc("/api/dashboard/stats", GetDashboardStats)

	// ── Training & Prediction (public read) ──────────────────────────
	mux.HandleFunc("/api/training/status", GetTrainingStatus)
	mux.HandleFunc("/api/training/history", GetTrainingHistory)
	mux.HandleFunc("/api/training/algorithms", GetTrainingAlgorithms)
	mux.HandleFunc("/api/training/metrics", GetTrainingMetrics)
	mux.HandleFunc("/api/training/", GetTrainingDetail)
	mux.HandleFunc("/api/predictions/direction-accuracy", AuthRequired(GetDirectionAccuracy))

	// ── Gold (auth + GOLD market access) ─────────────────────────────
	goldR := MarketRequired("GOLD")
	mux.HandleFunc("/api/gold/latest",                        AuthRequired(goldR(GetGoldLatest)))
	mux.HandleFunc("/api/gold/prices",                        AuthRequired(goldR(GetGoldPrices)))
	mux.HandleFunc("/api/gold/chart",                         AuthRequired(goldR(GetGoldChart)))
	mux.HandleFunc("/api/gold/predictions/latest-results",    AuthRequired(goldR(GetGoldPredictionsLatestResults)))
	mux.HandleFunc("/api/gold/predictions/latest",            AuthRequired(goldR(GetLatestGoldPredictions)))
	mux.HandleFunc("/api/gold/predictions/chart",             AuthRequired(goldR(GetGoldPredictionChart)))
	mux.HandleFunc("/api/gold/predictions",                   AuthRequired(goldR(GetGoldPredictions)))

	// ── NASDAQ (auth + NASDAQ market access) ─────────────────────────
	nasdaqR := MarketRequired("NASDAQ")
	mux.HandleFunc("/api/nasdaq/latest",                      AuthRequired(nasdaqR(GetNasdaqLatest)))
	mux.HandleFunc("/api/nasdaq/prices",                      AuthRequired(nasdaqR(GetNasdaqPrices)))
	mux.HandleFunc("/api/nasdaq/chart",                       AuthRequired(nasdaqR(GetNasdaqChart)))
	mux.HandleFunc("/api/nasdaq/predictions/latest-results",  AuthRequired(nasdaqR(GetNasdaqPredictionsLatestResults)))
	mux.HandleFunc("/api/nasdaq/predictions/latest",          AuthRequired(nasdaqR(GetNasdaqPredictionsLatest)))
	mux.HandleFunc("/api/nasdaq/predictions/chart",           AuthRequired(nasdaqR(GetNasdaqPredictionsChart)))
	mux.HandleFunc("/api/nasdaq/predictions",                 AuthRequired(nasdaqR(GetNasdaqPredictions)))

	// ── Crypto (auth + CRYPTO market access) ─────────────────────────
	cryptoR := MarketRequired("CRYPTO")
	mux.HandleFunc("/api/crypto/latest",                      AuthRequired(cryptoR(GetCryptoLatest)))
	mux.HandleFunc("/api/crypto/prices",                      AuthRequired(cryptoR(GetCryptoPrices)))
	mux.HandleFunc("/api/crypto/chart",                       AuthRequired(cryptoR(GetCryptoChart)))
	mux.HandleFunc("/api/crypto/predictions/latest-results",  AuthRequired(cryptoR(GetCryptoPredictionsLatestResults)))
	mux.HandleFunc("/api/crypto/predictions/latest",          AuthRequired(cryptoR(GetCryptoPredictionsLatest)))
	mux.HandleFunc("/api/crypto/predictions/chart",           AuthRequired(cryptoR(GetCryptoPredictionsChart)))
	mux.HandleFunc("/api/crypto/predictions",                 AuthRequired(cryptoR(GetCryptoPredictions)))

	// ── SP500 (auth + SP500 market access) ───────────────────────────
	sp500R := MarketRequired("SP500")
	mux.HandleFunc("/api/sp500/latest",                       AuthRequired(sp500R(GetSP500Latest)))
	mux.HandleFunc("/api/sp500/prices",                       AuthRequired(sp500R(GetSP500Prices)))
	mux.HandleFunc("/api/sp500/chart",                        AuthRequired(sp500R(GetSP500Chart)))
	mux.HandleFunc("/api/sp500/predictions/latest-results",   AuthRequired(sp500R(GetSP500PredictionsLatestResults)))
	mux.HandleFunc("/api/sp500/predictions/latest",           AuthRequired(sp500R(GetSP500PredictionsLatest)))
	mux.HandleFunc("/api/sp500/predictions/chart",            AuthRequired(sp500R(GetSP500PredictionsChart)))
	mux.HandleFunc("/api/sp500/predictions",                  AuthRequired(sp500R(GetSP500Predictions)))

	// ── Markets paginated (check per key in handler) ──────────────────
	mux.HandleFunc("/api/markets/{key}/predictions", AuthRequired(GetMarketPredictions))
	mux.HandleFunc("/api/markets/{key}/training",    AuthRequired(GetMarketTraining))

	// ── Triggers (admin or super_admin only) ─────────────────────────
	mux.HandleFunc("POST /api/trigger/gold-crawler",          AdminRequired(TriggerGoldCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/gold-history",          AdminRequired(TriggerGoldHistoryHandler))
	mux.HandleFunc("POST /api/trigger/gold-predict",          AdminRequired(TriggerGoldPredictHandler))
	mux.HandleFunc("POST /api/trigger/train",                 AdminRequired(TriggerTrainHandler))
	mux.HandleFunc("POST /api/trigger/reconcile",             AdminRequired(TriggerReconcileHandler))
	mux.HandleFunc("POST /api/trigger/historical-backtest",   AdminRequired(TriggerHistoricalBacktestHandler))
	mux.HandleFunc("POST /api/trigger/gold-historical-backtest", AdminRequired(TriggerGoldHistoricalBacktestHandler))
	mux.HandleFunc("POST /api/trigger/nasdaq-crawler",        AdminRequired(TriggerNasdaqCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/nasdaq-predict",        AdminRequired(TriggerNasdaqPredictHandler))
	mux.HandleFunc("POST /api/trigger/crypto-crawler",        AdminRequired(TriggerCryptoCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/crypto-predict",        AdminRequired(TriggerCryptoPredictHandler))
	mux.HandleFunc("POST /api/trigger/sp500-crawler",         AdminRequired(TriggerSP500CrawlerHandler))
	mux.HandleFunc("POST /api/trigger/sp500-predict",         AdminRequired(TriggerSP500PredictHandler))
	mux.HandleFunc("POST /api/trigger/simulation-backtest",   AdminRequired(TriggerSimulationBacktestHandler))
	mux.HandleFunc("POST /api/trigger/simulation-live-step",  AdminRequired(TriggerSimulationLiveStepHandler))
	mux.HandleFunc("POST /api/trigger/sim-reset",             AdminRequired(TriggerSimResetHandler))

	// ── Backup (JWT required; trigger/delete require admin) ───────────
	mux.HandleFunc("POST /api/trigger/backup",           AdminRequired(TriggerBackupHandler))
	mux.HandleFunc("GET /api/backups",                   AuthRequired(ListBackupsHandler))
	mux.HandleFunc("GET /api/backups/{filename}",        AuthRequired(DownloadBackupHandler))
	mux.HandleFunc("DELETE /api/backups/{filename}",     AdminRequired(DeleteBackupHandler))

	// ── User management (admin only — proxy to Java) ──────────────────
	mux.HandleFunc("GET /api/users",         AdminRequired(ListUsersHandler))
	mux.HandleFunc("POST /api/users",        AdminRequired(CreateUserHandler))
	mux.HandleFunc("DELETE /api/users/{id}", AdminRequired(DeleteUserHandler))

	// ── Market groups (admin only — proxy to Java) ────────────────────
	mux.HandleFunc("GET /api/market-groups",                        AdminRequired(ListMarketGroupsHandler))
	mux.HandleFunc("POST /api/market-groups",                       AdminRequired(CreateMarketGroupHandler))
	mux.HandleFunc("PUT /api/market-groups/{id}",                   AdminRequired(UpdateMarketGroupHandler))
	mux.HandleFunc("DELETE /api/market-groups/{id}",                AdminRequired(DeleteMarketGroupHandler))
	mux.HandleFunc("PUT /api/market-groups/{id}/markets",           AdminRequired(SetGroupMarketsHandler))
	mux.HandleFunc("GET /api/market-groups/{id}/users",             AdminRequired(ListGroupUsersHandler))
	mux.HandleFunc("POST /api/market-groups/{id}/users",            AdminRequired(AddUserToGroupHandler))
	mux.HandleFunc("DELETE /api/market-groups/{id}/users/{uid}",    AdminRequired(RemoveUserFromGroupHandler))
	mux.HandleFunc("GET /api/users/{id}/market-groups",             AuthRequired(GetUserMarketGroupsHandler))

	// ── Schedules (JWT required) ──────────────────────────────────────
	mux.HandleFunc("GET /api/schedules",      AuthRequired(GetSchedulesHandler))
	mux.HandleFunc("PUT /api/schedules/{key}", AuthRequired(UpdateScheduleHandler))

	// ── Simulation (auth required; filter by market access in handler) ─
	mux.HandleFunc("GET /api/simulation/leaderboard",      AuthRequired(GetSimLeaderboard))
	mux.HandleFunc("GET /api/simulation/bots",             AuthRequired(GetSimBots))
	mux.HandleFunc("GET /api/simulation/bots/{id}/trades", AuthRequired(GetSimBotTrades))
	mux.HandleFunc("GET /api/simulation/bots/{id}/chart",  AuthRequired(GetSimBotChart))
	mux.HandleFunc("PUT /api/simulation/bots/{id}/config", AdminRequired(UpdateSimBotConfig))
	mux.HandleFunc("POST /api/simulation/bots/{id}/toggle", AdminRequired(ToggleSimBot))
	mux.HandleFunc("POST /api/simulation/bots/{id}/run",   AdminRequired(TriggerSimBotRun))
	mux.HandleFunc("POST /api/simulation/run-all",         AdminRequired(TriggerSimRunAll))
	mux.HandleFunc("GET /api/simulation/bots/",            AuthRequired(GetSimBot))

	// ── Monitoring (auth required; filter in handler) ─────────────────
	mux.HandleFunc("GET /api/monitoring/overview", AuthRequired(GetMonitoringOverview))

	return mux
}

func SetupAllRoutes() *http.ServeMux {
	mux := addHandler()
	logRegisteredRoutes()
	return mux
}

func logRegisteredRoutes() {
	logger.Logger.Info("Routes registered — auth proxied to Java Auth Service :8120")
}
```

**Lưu ý:** Các handler `GetMarketPredictions` và `GetMarketTraining` cần được cập nhật để check market access từ JWT claims (xem bước tiếp theo).

- [ ] **Commit:**
```bash
git add api/pkg/server/router.go
git commit -m "feat(auth): update router — market routes require auth+market, triggers require admin"
```

---

## Task 10: GetMarketPredictions + GetMarketTraining — thêm market check

**Files:**
- Modify: `api/pkg/server/api_markets.go`

- [ ] **Trong handler `GetMarketPredictions` và `GetMarketTraining`, thêm check đầu hàm:**

```go
// Ví dụ trong GetMarketPredictions:
func GetMarketPredictions(w http.ResponseWriter, r *http.Request) {
    key := strings.ToUpper(r.PathValue("key"))
    // map path key → market key
    marketKey := pathToMarketKey(key) // "gold" → "GOLD", "nasdaq100" → "NASDAQ", v.v.

    claims := getClaims(r) // đã có vì AuthRequired wrap ở router
    if !isSuperAdmin(claims) {
        hasAccess := false
        for _, m := range getAccessibleMarkets(claims) {
            if m == marketKey {
                hasAccess = true
                break
            }
        }
        if !hasAccess {
            ResponseError(w, http.StatusForbidden, "no access to market: "+marketKey)
            return
        }
    }
    // ... rest of handler
}

// Helper để map URL path key sang market key constant
func pathToMarketKey(key string) string {
    switch strings.ToUpper(key) {
    case "GOLD":
        return "GOLD"
    case "NASDAQ", "NASDAQ100":
        return "NASDAQ"
    case "CRYPTO":
        return "CRYPTO"
    case "SP500":
        return "SP500"
    default:
        return strings.ToUpper(key)
    }
}
```

- [ ] **Kiểm tra compile toàn bộ:**
```bash
cd api && go build ./...
```

- [ ] **Commit:**
```bash
git add api/pkg/server/api_markets.go
git commit -m "feat(auth): add market access check in GetMarketPredictions and GetMarketTraining"
```

---

## Task 11: Dọn dẹp UserStore + AllModels

**Files:**
- Delete: `api/pkg/store/repository/user.go`
- Delete: `api/pkg/store/mysql/user.go`
- Modify: `api/pkg/store/repository/repository.go`
- Modify: `api/pkg/models/models_db/migrations.go`

- [ ] **Xóa files UserStore:**
```bash
rm api/pkg/store/repository/user.go
rm api/pkg/store/mysql/user.go
```

- [ ] **Sửa `api/pkg/store/repository/repository.go` — xóa `UserStore` khỏi `DatabaseStore` interface:**

Xóa dòng `UserStore` khỏi `DatabaseStore interface { ... }` và xóa type definition `UserStore interface { ... }`.

- [ ] **Sửa `api/pkg/models/models_db/migrations.go` — xóa `&User{}` khỏi `AllModels`:**

```go
var AllModels = []interface{}{
    &SyncLog{},
    &GoldPrice{},
    &GoldPrediction{},
    &MacroIndicator{},
    &TrainingLog{},
    // &User{} ← đã xóa — Java Flyway owns users table
    &CronSchedule{},
    &NasdaqPrice{},
    // ... các model khác giữ nguyên
}
```

- [ ] **Kiểm tra compile + fix any broken imports:**
```bash
cd api && go build ./...
```

- [ ] **Chạy Go tests để đảm bảo không break:**
```bash
cd api && go test ./... -short 2>&1 | tail -20
```

- [ ] **Commit:**
```bash
git add -A
git commit -m "feat(auth): remove UserStore from Go — Java owns users table"
```

---

## Task 12: Smoke test — build và chạy tích hợp

- [ ] **Build Go API binary:**
```bash
cd api && go build -o /tmp/api-server ./cmd/
```
Expected: không có lỗi.

- [ ] **Build lại Docker image Go API:**
```bash
docker build -t go-stock-prediction-api:latest api/
```

- [ ] **Commit final:**
```bash
git add -A
git commit -m "feat(auth): Go API auth proxy complete — market access, admin triggers, market groups"
```
