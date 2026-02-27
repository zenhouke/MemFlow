package vectorstore

import (
	"context"
)

type VectorStore interface {
	Add(ctx context.Context, vectors []VectorRecord) error
	Search(ctx context.Context, query []float32, topK int, filter *Filter) ([]SearchResult, error)
	Delete(ctx context.Context, ids []string) error
	Upsert(ctx context.Context, vectors []VectorRecord) error
	Close() error
}

type VectorRecord struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}

type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

type Filter struct {
	Must   []Condition
	Should []Condition
}

type Condition struct {
	Field    string
	Operator string
	Value    interface{}
}

const (
	OpEqual = "eq"
	OpIn    = "in"
	OpRange = "range"
)
