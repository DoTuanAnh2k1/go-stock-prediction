package grpcserver

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/predict"
	goldpredict "go-stock-prediction/pkg/service/predict/gold"
	"go-stock-prediction/pkg/service/predict/orchestrator"
	pb "go-stock-prediction/proto/prediction"
)

// Server implements pb.PredictionServiceServer.
type Server struct {
	pb.UnimplementedPredictionServiceServer

	grpcServer       *grpc.Server
	backtestRunning  atomic.Int32
}

// New creates a new gRPC server instance.
func New() *Server {
	return &Server{}
}

// Start binds the gRPC server to the given port and begins serving.
// The call blocks until the listener is established; actual serving
// happens in a background goroutine.
func (s *Server) Start(port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("grpc server: failed to listen on port %s: %w", port, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterPredictionServiceServer(s.grpcServer, s)

	logger.Logger.Infof("gRPC prediction server listening on :%s", port)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			logger.Logger.Errorf("gRPC server stopped: %v", err)
		}
	}()

	return nil
}

// Stop performs a graceful shutdown of the gRPC server.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		logger.Logger.Info("Stopping gRPC prediction server…")
		s.grpcServer.GracefulStop()
	}
}

// ---------------------------------------------------------------------------
// RPC implementations
// ---------------------------------------------------------------------------

// TriggerCrawler fires the VN30 stock crawler in a goroutine and returns immediately.
func (s *Server) TriggerCrawler(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerCrawler called")
	go func() {
		if err := crawler.CronjobCrawler(); err != nil {
			logger.Logger.Errorf("[gRPC] CronjobCrawler error: %v", err)
		}
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Stock crawler started in background",
	}, nil
}

// TriggerGoldCrawler runs the gold crawler synchronously and returns the result.
func (s *Server) TriggerGoldCrawler(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerGoldCrawler called")
	if err := crawler.CronjobGoldCrawler(); err != nil {
		logger.Logger.Errorf("[gRPC] CronjobGoldCrawler error: %v", err)
		return &pb.TriggerResponse{
			Success: false,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "gold crawler failed: %v", err)
	}
	return &pb.TriggerResponse{
		Success: true,
		Message: "Gold crawler completed successfully",
	}, nil
}

// TriggerPredict runs weekly training then daily prediction synchronously.
func (s *Server) TriggerPredict(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerPredict called")

	if err := predict.CronjobWeeklyTraining(); err != nil {
		logger.Logger.Errorf("[gRPC] CronjobWeeklyTraining error: %v", err)
		return &pb.TriggerResponse{
			Success: false,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "weekly training failed: %v", err)
	}

	if err := predict.CronjobDailyPrediction(); err != nil {
		logger.Logger.Errorf("[gRPC] CronjobDailyPrediction error: %v", err)
		return &pb.TriggerResponse{
			Success: false,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "daily prediction failed: %v", err)
	}

	return &pb.TriggerResponse{
		Success: true,
		Message: "Training and prediction completed successfully",
	}, nil
}

// TriggerTrain trains a single algorithm or all algorithms depending on the request.
func (s *Server) TriggerTrain(_ context.Context, req *pb.TriggerTrainRequest) (*pb.TriggerTrainResponse, error) {
	logger.Logger.Infof("[gRPC] TriggerTrain called (algorithm=%q)", req.GetAlgorithm())

	if predict.IsTraining() {
		return &pb.TriggerTrainResponse{
			Success: false,
			Error:   "training already in progress",
		}, status.Error(codes.Aborted, "training already in progress")
	}

	var (
		sessionID string
		err       error
	)

	if req.GetAlgorithm() != "" {
		sessionID, err = predict.TrainSingleAlgorithmByName(req.GetAlgorithm())
	} else {
		sessionID, err = predict.TrainAllAlgorithms()
	}

	if err != nil {
		logger.Logger.Errorf("[gRPC] TriggerTrain error: %v", err)
		return &pb.TriggerTrainResponse{
			Success: false,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "training failed: %v", err)
	}

	return &pb.TriggerTrainResponse{
		Success:   true,
		SessionId: sessionID,
		Message:   "Training started",
	}, nil
}

// TriggerReconcile runs prediction reconciliation synchronously.
func (s *Server) TriggerReconcile(ctx context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerReconcile called")

	if err := predict.ReconcilePredictions(ctx); err != nil {
		logger.Logger.Errorf("[gRPC] ReconcilePredictions error: %v", err)
		return &pb.TriggerResponse{
			Success: false,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "reconcile failed: %v", err)
	}

	return &pb.TriggerResponse{
		Success: true,
		Message: "Prediction reconciliation completed",
	}, nil
}

// TriggerStockHistory starts a historical stock price crawl in a goroutine and returns immediately.
func (s *Server) TriggerStockHistory(ctx context.Context, req *pb.StockHistoryRequest) (*pb.TriggerResponse, error) {
	days := int(req.GetDays())
	logger.Logger.Infof("[gRPC] TriggerStockHistory called (days=%d)", days)

	go func() {
		bgCtx := context.Background()
		saved, skipped, err := crawler.CrawlHistoricalAll(bgCtx, days)
		if err != nil {
			logger.Logger.Errorf("[gRPC] CrawlHistoricalAll error: %v", err)
			return
		}
		logger.Logger.Infof("[gRPC] CrawlHistoricalAll done: saved=%d skipped=%d", saved, skipped)
	}()

	return &pb.TriggerResponse{
		Success: true,
		Message: fmt.Sprintf("Historical stock crawl started in background (days=%d)", days),
	}, nil
}

// TriggerGoldHistory fires the XAU history import as fire-and-forget.
func (s *Server) TriggerGoldHistory(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerGoldHistory called")
	go crawler.ImportXAUHistory()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Gold history import started in background",
	}, nil
}

// TriggerGoldPredict runs gold prediction in a goroutine and returns immediately.
func (s *Server) TriggerGoldPredict(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerGoldPredict called")
	go func() {
		count, err := goldpredict.RunNow()
		if err != nil {
			logger.Logger.Errorf("[gRPC] goldpredict.RunNow error: %v", err)
			return
		}
		logger.Logger.Infof("[gRPC] goldpredict.RunNow done: %d predictions saved", count)
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Gold prediction started in background",
	}, nil
}

// TriggerHistoricalBacktest starts a walk-forward backtest in a goroutine.
// Returns an error if a backtest is already running.
func (s *Server) TriggerHistoricalBacktest(_ context.Context, req *pb.BacktestRequest) (*pb.TriggerResponse, error) {
	trainWindow := int(req.GetTrainWindow())
	stepSize := int(req.GetStepSize())
	logger.Logger.Infof("[gRPC] TriggerHistoricalBacktest called (trainWindow=%d, stepSize=%d)", trainWindow, stepSize)

	// trainWindow < 0 is the signal for gold historical backtest.
	if trainWindow < 0 {
		if !s.backtestRunning.CompareAndSwap(0, 1) {
			return &pb.TriggerResponse{
				Success: false,
				Error:   "backtest already running",
			}, status.Error(codes.Aborted, "backtest already running")
		}
		go func() {
			defer s.backtestRunning.Store(0)
			bgCtx := context.Background()
			result, err := goldpredict.RunGoldHistoricalBacktest(bgCtx, 0, 0) // uses defaults
			if err != nil {
				logger.Logger.Errorf("[gRPC] RunGoldHistoricalBacktest error: %v", err)
				return
			}
			logger.Logger.Infof("[gRPC] RunGoldHistoricalBacktest done: %d predictions, %d instruments in %dms",
				result.TotalPredictions, result.StocksProcessed, result.DurationMs)
		}()
		return &pb.TriggerResponse{
			Success: true,
			Message: "Gold historical backtest started in background",
		}, nil
	}

	if !s.backtestRunning.CompareAndSwap(0, 1) {
		return &pb.TriggerResponse{
			Success: false,
			Error:   "backtest already running",
		}, status.Error(codes.Aborted, "backtest already running")
	}

	go func() {
		defer s.backtestRunning.Store(0)

		bgCtx := context.Background()
		result, err := predict.RunHistoricalBacktest(bgCtx, trainWindow, stepSize)
		if err != nil {
			logger.Logger.Errorf("[gRPC] RunHistoricalBacktest error: %v", err)
			return
		}
		logger.Logger.Infof(
			"[gRPC] RunHistoricalBacktest done: %d predictions, %d stocks, %d algos in %dms",
			result.TotalPredictions, result.StocksProcessed, result.AlgorithmsRun, result.DurationMs,
		)
	}()

	return &pb.TriggerResponse{
		Success: true,
		Message: "Historical backtest started in background",
	}, nil
}

// TriggerStockCrawl crawls a single stock by symbol and saves it synchronously.
func (s *Server) TriggerStockCrawl(ctx context.Context, req *pb.StockRequest) (*pb.StockCrawlResponse, error) {
	symbol := req.GetSymbol()
	logger.Logger.Infof("[gRPC] TriggerStockCrawl called (symbol=%s)", symbol)

	stockData, err := crawler.CrawlAndSaveSingleStock(ctx, symbol)
	if err != nil {
		logger.Logger.Errorf("[gRPC] CrawlAndSaveSingleStock(%s) error: %v", symbol, err)
		return &pb.StockCrawlResponse{
			Success: false,
			Symbol:  symbol,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "stock crawl failed: %v", err)
	}

	return &pb.StockCrawlResponse{
		Success: true,
		Symbol:  stockData.Symbol,
		Message: fmt.Sprintf("Stock %s crawled and saved successfully", symbol),
	}, nil
}

// TriggerStockPredict runs all prediction algorithms for a single stock synchronously.
func (s *Server) TriggerStockPredict(_ context.Context, req *pb.StockRequest) (*pb.StockPredictResponse, error) {
	symbol := req.GetSymbol()
	logger.Logger.Infof("[gRPC] TriggerStockPredict called (symbol=%s)", symbol)

	count, err := predict.PredictSingleStock(symbol)
	if err != nil {
		logger.Logger.Errorf("[gRPC] PredictSingleStock(%s) error: %v", symbol, err)
		return &pb.StockPredictResponse{
			Success: false,
			Symbol:  symbol,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "stock prediction failed: %v", err)
	}

	return &pb.StockPredictResponse{
		Success:          true,
		Symbol:           symbol,
		PredictionsCount: int32(count),
		Message:          fmt.Sprintf("Generated %d predictions for %s", count, symbol),
	}, nil
}

// TriggerNasdaqCrawler fires the NASDAQ crawler in a goroutine and returns immediately.
func (s *Server) TriggerNasdaqCrawler(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerNasdaqCrawler called")
	go func() {
		if err := crawler.CronjobNasdaqCrawler(); err != nil {
			logger.Logger.Errorf("[gRPC] CronjobNasdaqCrawler error: %v", err)
		}
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "NASDAQ crawler started in background",
	}, nil
}

// TriggerNasdaqPredict runs NASDAQ predictions via the orchestrator in a goroutine.
func (s *Server) TriggerNasdaqPredict(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerNasdaqPredict called")
	go func() {
		n, err := orchestrator.RunForMarket(context.Background(), "NASDAQ100")
		if err != nil {
			logger.Logger.Errorf("[gRPC] NASDAQ prediction error: %v", err)
			return
		}
		logger.Logger.Infof("[gRPC] NASDAQ prediction done: %d predictions saved", n)
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "NASDAQ prediction started in background",
	}, nil
}

// TriggerCryptoCrawler fires the crypto crawler in a goroutine and returns immediately.
func (s *Server) TriggerCryptoCrawler(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerCryptoCrawler called")
	go func() {
		if err := crawler.CronjobCryptoCrawler(); err != nil {
			logger.Logger.Errorf("[gRPC] CronjobCryptoCrawler error: %v", err)
		}
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Crypto crawler started in background",
	}, nil
}

// TriggerCryptoPredict runs crypto predictions via the orchestrator in a goroutine.
func (s *Server) TriggerCryptoPredict(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerCryptoPredict called")
	go func() {
		n, err := orchestrator.RunForMarket(context.Background(), "CRYPTO")
		if err != nil {
			logger.Logger.Errorf("[gRPC] Crypto prediction error: %v", err)
			return
		}
		logger.Logger.Infof("[gRPC] Crypto prediction done: %d predictions saved", n)
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Crypto prediction started in background",
	}, nil
}

// TriggerFuelCrawler fires the fuel crawler in a goroutine and returns immediately.
func (s *Server) TriggerFuelCrawler(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerFuelCrawler called")
	go func() {
		if err := crawler.CronjobFuelCrawler(); err != nil {
			logger.Logger.Errorf("[gRPC] CronjobFuelCrawler error: %v", err)
		}
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Fuel crawler started in background",
	}, nil
}

// TriggerFuelPredict runs fuel predictions via the orchestrator in a goroutine.
func (s *Server) TriggerFuelPredict(_ context.Context, _ *pb.Empty) (*pb.TriggerResponse, error) {
	logger.Logger.Info("[gRPC] TriggerFuelPredict called")
	go func() {
		n, err := orchestrator.RunForMarket(context.Background(), "FUEL")
		if err != nil {
			logger.Logger.Errorf("[gRPC] Fuel prediction error: %v", err)
			return
		}
		logger.Logger.Infof("[gRPC] Fuel prediction done: %d predictions saved", n)
	}()
	return &pb.TriggerResponse{
		Success: true,
		Message: "Fuel prediction started in background",
	}, nil
}

// GetTrainingStatus returns the current training state of the prediction service.
func (s *Server) GetTrainingStatus(_ context.Context, _ *pb.Empty) (*pb.TrainingStatusResponse, error) {
	isTraining := predict.IsTraining()
	lastTrained := predict.GetLastTrainedTime()

	var lastTrainedStr string
	if !lastTrained.IsZero() {
		lastTrainedStr = lastTrained.UTC().Format(time.RFC3339)
	}

	// Pull live progress fields from the singleton when available.
	var progress float64
	var phase string
	var totalAlgos, doneAlgos int32

	svc := predict.GetPredictionService()
	if svc != nil {
		p, ph, total, done := svc.TrainingProgress()
		progress = p
		phase = ph
		totalAlgos = int32(total)
		doneAlgos = int32(done)
	}

	if phase == "" {
		if isTraining {
			phase = "training"
		} else {
			phase = "idle"
		}
	}

	return &pb.TrainingStatusResponse{
		IsTraining:      isTraining,
		LastTrained:     lastTrainedStr,
		Progress:        progress,
		CurrentPhase:    phase,
		TotalAlgorithms: totalAlgos,
		DoneAlgorithms:  doneAlgos,
	}, nil
}
