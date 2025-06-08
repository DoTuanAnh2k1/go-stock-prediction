package server

import (
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/predict"
	"net/http"
)

func TriggerPredictHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger predict")
	logger.Logger.Info("Trainning")
	err := predict.CronjobWeeklyTraining()
	if err != nil {
		logger.Logger.Errorf("Trainning error: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logger.Logger.Info("Predict")
	err = predict.CronjobDailyPrediction()
	if err != nil {
		logger.Logger.Errorf("Predict error: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
