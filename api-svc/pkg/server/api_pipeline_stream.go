package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/grpc/client"
	pb "go-stock-prediction/proto/prediction"
)

// sseEvent writes a single SSE event to w and flushes.
func sseEvent(w http.ResponseWriter, flusher http.Flusher, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}

// validateTokenParam checks a JWT passed as ?token= query param.
// Returns (role, ok). Needed because EventSource doesn't support custom headers.
func validateTokenParam(r *http.Request) (role string, ok bool) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		return "", false
	}
	cfg := config.GetServerConfig()
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return "", false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}
	role, _ = claims["role"].(string)
	return role, true
}

// StreamPipelinePredictHandler godoc
//
//	@Summary      Stream live predict logs via SSE
//	@Description  Opens a Server-Sent Events connection and streams per-symbol/algo progress
//	              while the prediction pipeline runs. Auth via ?token= query param (admin only).
//	@Tags         Triggers
//	@Produce      text/event-stream
//	@Param        market  query  string  true  "Market key: gold, nasdaq, crypto, sp500"
//	@Param        token   query  string  true  "JWT bearer token"
//	@Success      200
//	@Failure      401  {object}  ResponseFailure
//	@Failure      403  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Router       /api/pipeline/stream [get]
func StreamPipelinePredictHandler(w http.ResponseWriter, r *http.Request) {
	// Auth: token comes as query param because EventSource doesn't support headers
	role, ok := validateTokenParam(r)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if role != "admin" && role != "super_admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	market := strings.ToUpper(r.URL.Query().Get("market"))
	if market == "" {
		http.Error(w, `{"error":"market param required"}`, http.StatusBadRequest)
		return
	}

	grpcClient := client.GetClient()
	if grpcClient == nil {
		http.Error(w, `{"error":"prediction service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var stream interface {
		Recv() (*pb.PipelineLogEvent, error)
	}
	var err error

	switch market {
	case "GOLD":
		stream, err = grpcClient.StreamGoldPredict(ctx, &pb.Empty{})
	case "NASDAQ100", "NASDAQ":
		stream, err = grpcClient.StreamNasdaqPredict(ctx, &pb.Empty{})
	case "CRYPTO":
		stream, err = grpcClient.StreamCryptoPredict(ctx, &pb.Empty{})
	case "SP500":
		stream, err = grpcClient.StreamSP500Predict(ctx, &pb.Empty{})
	default:
		sseEvent(w, flusher, map[string]any{"level": "error", "msg": "unknown market: " + market, "done": true})
		return
	}

	if err != nil {
		sseEvent(w, flusher, map[string]any{"level": "error", "msg": err.Error(), "done": true})
		return
	}

	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			sseEvent(w, flusher, map[string]any{"level": "ok", "msg": "stream ended", "done": true, "progress": 1.0})
			return
		}
		if err != nil {
			sseEvent(w, flusher, map[string]any{"level": "error", "msg": err.Error(), "done": true})
			return
		}
		sseEvent(w, flusher, map[string]any{
			"level":    evt.Level,
			"msg":      evt.Msg,
			"progress": evt.Progress,
			"done":     evt.Done,
			"error":    evt.Error,
		})
		if evt.Done {
			return
		}
	}
}
