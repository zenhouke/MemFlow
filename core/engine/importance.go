package engine

import "context"

type ImportanceEstimator interface {
	Estimate(ctx context.Context, content string) (float64, error)
}
