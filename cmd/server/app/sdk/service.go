// Package sdk defines the server application boundary.
package sdk

import (
	"context"

	"github.com/ardanlabs/malina/sdk/malina/model"
)

// ModelInfo describes the resident model and its current activity.
type ModelInfo struct {
	Path              string `json:"model_path"`
	QueueDepth        int    `json:"queue_depth"`
	ActiveGenerations int    `json:"active_generations"`
}

// Service is the model lifecycle used by HTTP transports.
type Service interface {
	Generate(context.Context, model.GenerateParams) (model.GeneratedImage, error)
	Load(context.Context, string, int) error
	Unload(context.Context) error
	Status() (ModelInfo, bool)
}
