package config

import "fmt"

type CompressionConfig struct {
	WindowSize  int
	OverlapSize int
}

type HybridSearchConfig struct {
	SemanticWeight float64
	LexicalWeight  float64
	SymbolicWeight float64

	EnableAdaptive bool
	BaseK          int
	Delta          float64
	MinK           int
	MaxK           int
}
type VectorStoreConfig struct {
	Type           string            // "memory", "qdrant", "lancedb", "milvus"
	Host           string            // 服务地址
	Port           int               // 端口
	CollectionName string            // 集合名称
	Dimension      int               // 向量维度
	APIKey         string            // API 密钥（可选）
	DBPath         string            // 本地数据库路径（LanceDB用）
	Extra          map[string]string // 额外配置
}

func (c *VectorStoreConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *VectorStoreConfig) Validate() error {
	if c.CollectionName == "" {
		return fmt.Errorf("collection name is required")
	}
	if c.Dimension <= 0 {
		return fmt.Errorf("dimension must be positive")
	}
	return nil
}
type Config struct {
	Alpha float64
	Beta  float64
	Gamma float64

	ShortTermDecay float64
	LongTermDecay  float64

	TopK int

	LongTermImportanceThreshold float64
	CompactionThreshold         int
	MergeSimilarityThreshold    float64

	EnableHybridSearch bool
	HybridSearchConfig HybridSearchConfig

	CompressionConfig CompressionConfig


	VectorStoreConfig VectorStoreConfig
}

func DefaultConfig() Config {
	return Config{
		Alpha: 0.6,
		Beta:  0.3,
		Gamma: 0.1,

		ShortTermDecay: 0.02,
		LongTermDecay:  0.005,

		TopK: 5,

		LongTermImportanceThreshold: 0.7,
		CompactionThreshold:         10,
		MergeSimilarityThreshold:    0.85,

		EnableHybridSearch: true,
		HybridSearchConfig: HybridSearchConfig{
			SemanticWeight: 0.6,
			LexicalWeight:  0.3,
			SymbolicWeight: 0.1,
			EnableAdaptive: true,
			BaseK:          5,
			Delta:          2.0,
			MinK:           3,
			MaxK:           20,
		},

		CompressionConfig: CompressionConfig{
			WindowSize:  10,
			OverlapSize: 2,
		},

		VectorStoreConfig: VectorStoreConfig{
			Type:           "memory",
			CollectionName: "default",
			Dimension:      1536,
		},
	}
}
