package modelsapi

import "time"

type TrainingStatusDTO struct {
	IsTraining    bool      `json:"is_training"`
	LastTrained   time.Time `json:"last_trained"`
	NextTraining  time.Time `json:"next_training"`
	CurrentPhase  string    `json:"current_phase"`
	Progress      float64   `json:"progress"`
	EstimatedTime string    `json:"estimated_time,omitempty"`
}
