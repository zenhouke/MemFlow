package engine

import (
	"simplemem/core/index"
	"simplemem/core/utils"
	"sync"
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

	mu sync.RWMutex // 细粒度锁：保护当前空间下的数据和索引

	IsCompacting bool // 标记位：表示当前空间是否正在进行后台压缩
}

func (s *MemorySpace) Init() {
	s.shortIndex = hnsw.NewGraph[string]()
	s.longIndex = hnsw.NewGraph[string]()
	s.shortBM25 = index.NewBM25Index()
	s.longBM25 = index.NewBM25Index()
	s.shortMetadata = index.NewMetadataIndex()
	s.longMetadata = index.NewMetadataIndex()

	if s.ShortTerm == nil {
		s.ShortTerm = make([]*MemoryItem, 0)
	}
	if s.LongTerm == nil {
		s.LongTerm = make([]*MemoryItem, 0)
	}
	if s.Archived == nil {
		s.Archived = make([]*MemoryItem, 0)
	}
}

// RebuildIndex 根据当前内存中的项重新构建所有索引
func (s *MemorySpace) RebuildIndex() {
	s.Init()

	for _, item := range s.ShortTerm {
		s.shortIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
		s.shortBM25.Add(item.ID, item.Content)
		s.shortMetadata.Add(item.ID, item.Metadata)
	}

	for _, item := range s.LongTerm {
		s.longIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
		s.longBM25.Add(item.ID, item.Content)
		s.longMetadata.Add(item.ID, item.Metadata)
	}
}

type HNSWIndex struct {
	graph *hnsw.Graph[string]
}

type TimeIndexEntry struct {
	DocID     int
	Timestamp time.Time
}
