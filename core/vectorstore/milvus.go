package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"memflow/core/config"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func init() {
	Register("milvus", func(cfg config.VectorStoreConfig) (VectorStore, error) {
		return NewMilvus(cfg)
	})
}
func NewMilvus(cfg config.VectorStoreConfig) (*MilvusStore, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 19530
	}

	addr := cfg.Address()

	c, err := client.NewClient(context.Background(), client.Config{
		Address: addr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to milvus at %s: %w", addr, err)
	}

	store := &MilvusStore{
		client:         c,
		collectionName: cfg.CollectionName,
		dimension:      cfg.Dimension,
	}

	if err := store.ensureCollection(context.Background()); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	return store, nil
}

type MilvusStore struct {
	client         client.Client
	collectionName string
	dimension      int
}


func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (m *MilvusStore) ensureCollection(ctx context.Context) error {
	exists, err := m.client.HasCollection(ctx, m.collectionName)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}

	if exists {
		// 加载集合以便搜索
		err = m.client.LoadCollection(ctx, m.collectionName, false)
		if err != nil {
			return fmt.Errorf("failed to load collection: %w", err)
		}
		return nil
	}

	// 创建集合 Schema
	schema := &entity.Schema{
		CollectionName: m.collectionName,
		Description:    "vectorstore collection",
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{
					"max_length": "256",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", m.dimension),
				},
			},
			{
				Name:     "payload",
				DataType: entity.FieldTypeJSON,
			},
		},
	}

	err = m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// 创建索引
	idx, err := entity.NewIndexIvfFlat(entity.COSINE, 128)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	err = m.client.CreateIndex(ctx, m.collectionName, "vector", idx, false)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	// 加载集合
	err = m.client.LoadCollection(ctx, m.collectionName, false)
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	return nil
}

func (m *MilvusStore) Add(ctx context.Context, vectors []VectorRecord) error {
	return m.insertData(ctx, vectors)
}

func (m *MilvusStore) Upsert(ctx context.Context, vectors []VectorRecord) error {
	return m.insertData(ctx, vectors)
}

func (m *MilvusStore) insertData(ctx context.Context, vectors []VectorRecord) error {
	if len(vectors) == 0 {
		return nil
	}

	ids := make([]string, len(vectors))
	vecs := make([][]float32, len(vectors))
	payloads := make([][]byte, len(vectors))

	for i, v := range vectors {
		ids[i] = v.ID
		vecs[i] = v.Vector

		// 将 payload 序列化为 JSON
		payloadBytes, err := marshalJSON(v.Payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload for %s: %w", v.ID, err)
		}
		payloads[i] = payloadBytes
	}

	idColumn := entity.NewColumnVarChar("id", ids)
	vectorColumn := entity.NewColumnFloatVector("vector", m.dimension, vecs)
	payloadColumn := entity.NewColumnJSONBytes("payload", payloads)

	_, err := m.client.Upsert(ctx, m.collectionName, "", idColumn, vectorColumn, payloadColumn)
	if err != nil {
		return fmt.Errorf("failed to upsert data: %w", err)
	}

	return nil
}

func (m *MilvusStore) Search(ctx context.Context, query []float32, topK int, filter *Filter) ([]SearchResult, error) {
	sp, err := entity.NewIndexIvfFlatSearchParam(16)
	if err != nil {
		return nil, fmt.Errorf("failed to create search params: %w", err)
	}

	vectors := []entity.Vector{entity.FloatVector(query)}

	expr := ""
	if filter != nil {
		expr = buildMilvusFilter(filter)
	}

	searchResult, err := m.client.Search(
		ctx,
		m.collectionName,
		nil,
		expr,
		[]string{"id", "payload"},
		vectors,
		"vector",
		entity.COSINE,
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	var results []SearchResult
	for _, sr := range searchResult {
		for i := 0; i < sr.ResultCount; i++ {
			id, _ := sr.IDs.GetAsString(i)
			score := sr.Scores[i]

			payload := make(map[string]interface{})
			// 从 payload 字段反序列化
			if payloadCol, ok := sr.Fields.GetColumn("payload").(*entity.ColumnJSONBytes); ok {
				if data, err := payloadCol.GetAsString(i); err == nil {
					unmarshalJSON([]byte(data), &payload)
				}
			}

			results = append(results, SearchResult{
				ID:      id,
				Score:   score,
				Payload: payload,
			})
		}
	}

	return results, nil
}

func (m *MilvusStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf(`"%s"`, id)
	}
	expr := fmt.Sprintf("id in [%s]", strings.Join(quoted, ","))

	err := m.client.Delete(ctx, m.collectionName, "", expr)
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

func (m *MilvusStore) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// ========== Milvus 过滤器构建 ==========

func buildMilvusFilter(filter *Filter) string {
	var parts []string

	for _, cond := range filter.Must {
		if expr := buildMilvusCondition(cond); expr != "" {
			parts = append(parts, expr)
		}
	}

	mustExpr := ""
	if len(parts) > 0 {
		mustExpr = strings.Join(parts, " and ")
	}

	var shouldParts []string
	for _, cond := range filter.Should {
		if expr := buildMilvusCondition(cond); expr != "" {
			shouldParts = append(shouldParts, expr)
		}
	}

	shouldExpr := ""
	if len(shouldParts) > 0 {
		shouldExpr = "(" + strings.Join(shouldParts, " or ") + ")"
	}

	if mustExpr != "" && shouldExpr != "" {
		return mustExpr + " and " + shouldExpr
	}
	if mustExpr != "" {
		return mustExpr
	}
	return shouldExpr
}

func buildMilvusCondition(cond Condition) string {
	// Milvus 中 JSON 字段访问用 payload["field"] 格式
	field := fmt.Sprintf(`payload["%s"]`, cond.Field)

	switch cond.Operator {
	case OpEqual:
		switch v := cond.Value.(type) {
		case string:
			return fmt.Sprintf(`%s == "%s"`, field, v)
		default:
			return fmt.Sprintf(`%s == %v`, field, v)
		}

	case OpIn:
		if values, ok := cond.Value.([]string); ok {
			quoted := make([]string, len(values))
			for i, v := range values {
				quoted[i] = fmt.Sprintf(`"%s"`, v)
			}
			return fmt.Sprintf(`%s in [%s]`, field, strings.Join(quoted, ","))
		}

	case OpRange:
		if rangeVal, ok := cond.Value.(map[string]interface{}); ok {
			var rangeParts []string
			if gte, ok := rangeVal["gte"]; ok {
				rangeParts = append(rangeParts, fmt.Sprintf(`%s >= %v`, field, gte))
			}
			if lte, ok := rangeVal["lte"]; ok {
				rangeParts = append(rangeParts, fmt.Sprintf(`%s <= %v`, field, lte))
			}
			if gt, ok := rangeVal["gt"]; ok {
				rangeParts = append(rangeParts, fmt.Sprintf(`%s > %v`, field, gt))
			}
			if lt, ok := rangeVal["lt"]; ok {
				rangeParts = append(rangeParts, fmt.Sprintf(`%s < %v`, field, lt))
			}
			return strings.Join(rangeParts, " and ")
		}
	}

	return ""
}
