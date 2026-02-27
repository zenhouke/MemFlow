package engine

import (
	"simplemem/core/index"
	"time"

	"github.com/coder/hnsw"
)

type MemoryItem struct {
	ID              string         `json:"id"`
	Content         string         `json:"content"`
	OriginalContent string         `json:"original_content"`
	Embedding       []float64      `json:"embedding"`
	Importance      float64        `json:"importance"`
	CreatedAt       time.Time      `json:"created_at"`
	LastAccessedAt  time.Time      `json:"last_accessed_at"`
	Metadata        index.Metadata `json:"metadata"`
}

type MemorySpace struct {
	ShortTerm []*MemoryItem `json:"short_term"`
	LongTerm  []*MemoryItem `json:"long_term"`
	Archived  []*MemoryItem `json:"archived"`

	shortIndex *hnsw.Graph[string]
	longIndex  *hnsw.Graph[string]

	shortBM25 *index.BM25Index
	longBM25  *index.BM25Index

	shortMetadata *index.MetadataIndex
	longMetadata  *index.MetadataIndex
}

type HNSWIndex struct {
	graph *hnsw.Graph[string]
}

type TimeIndexEntry struct {
	DocID     int
	Timestamp time.Time
}
