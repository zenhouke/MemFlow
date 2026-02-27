// vectorstore/factory.go
package vectorstore

import (
	"context"
	"fmt"
	"net/http"
	"simplemem/core/config"
	"time"
)

type FactoryFunc func(cfg config.VectorStoreConfig) (VectorStore, error)

var registry = map[string]FactoryFunc{}

// Register 注册一个向量存储的工厂方法
func Register(name string, factory FactoryFunc) {
	registry[name] = factory
}

// New 根据配置创建向量存储实例
func New(cfg config.VectorStoreConfig) (VectorStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	factory, ok := registry[cfg.Type]
	if !ok {
		available := make([]string, 0, len(registry))
		for k := range registry {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown vector store type %q, available: %v", cfg.Type, available)
	}

	return factory(cfg)
}





func NewLancedb(cfg config.VectorStoreConfig) (*LanceDBStore, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	store := &LanceDBStore{
		baseURL:   fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		tableName: cfg.CollectionName,
		dimension: cfg.Dimension,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 确保表存在
	if err := store.ensureTable(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}