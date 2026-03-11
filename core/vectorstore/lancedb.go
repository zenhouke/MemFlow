package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"memflow/core/config"
	"sort"
	"strings"
	"sync"
)

func init() {
	Register("lancedb", func(cfg config.VectorStoreConfig) (VectorStore, error) {
		return NewLancedb(cfg)
	})
}

// LanceDBStore 使用嵌入式方式（本地文件存储 + 内存索引）
// 注意：Go 原生没有 LanceDB SDK，这里提供两种策略：
// 1. 通过 REST API 对接 LanceDB Cloud / LanceDB Server
// 2. 嵌入式简化实现（用于开发测试）
// 这里实现 REST API 方式
type LanceDBStore struct {
	baseURL        string
	tableName      string
	dimension      int
	httpClient     *http.Client
	mu             sync.RWMutex
}



func (l *LanceDBStore) ensureTable(ctx context.Context) error {
	createReq := map[string]interface{}{
		"name":      l.tableName,
		"dimension": l.dimension,
	}

	body, _ := json.Marshal(createReq)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/table/%s/create", l.baseURL, l.tableName), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	defer resp.Body.Close()

	// 忽略 409 已存在的错误
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create table, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (l *LanceDBStore) Add(ctx context.Context, vectors []VectorRecord) error {
	return l.upsertRecords(ctx, vectors)
}

func (l *LanceDBStore) Upsert(ctx context.Context, vectors []VectorRecord) error {
	return l.upsertRecords(ctx, vectors)
}

func (l *LanceDBStore) upsertRecords(ctx context.Context, vectors []VectorRecord) error {
	records := make([]map[string]interface{}, len(vectors))
	for i, v := range vectors {
		record := map[string]interface{}{
			"id":     v.ID,
			"vector": v.Vector,
		}
		for k, val := range v.Payload {
			record[k] = val
		}
		records[i] = record
	}

	reqBody := map[string]interface{}{
		"records": records,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/table/%s/add", l.baseURL, l.tableName), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add records: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add records, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (l *LanceDBStore) Search(ctx context.Context, query []float32, topK int, filter *Filter) ([]SearchResult, error) {
	searchReq := map[string]interface{}{
		"vector": query,
		"k":      topK,
	}

	if filter != nil {
		searchReq["filter"] = buildLanceDBFilter(filter)
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/table/%s/search", l.baseURL, l.tableName), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var searchResp struct {
		Results []struct {
			ID       string                 `json:"id"`
			Score    float32                `json:"_distance"`
			Payload  map[string]interface{} `json:"payload"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]SearchResult, len(searchResp.Results))
	for i, r := range searchResp.Results {
		results[i] = SearchResult{
			ID:      r.ID,
			Score:   1.0 - r.Score, // LanceDB 返回距离，转为相似度
			Payload: r.Payload,
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (l *LanceDBStore) Delete(ctx context.Context, ids []string) error {
	reqBody := map[string]interface{}{
		"ids": ids,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1/table/%s/delete", l.baseURL, l.tableName), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (l *LanceDBStore) Close() error {
	l.httpClient.CloseIdleConnections()
	return nil
}

// buildLanceDBFilter 构建 LanceDB SQL-like 过滤表达式
func buildLanceDBFilter(filter *Filter) string {
	var parts []string

	for _, cond := range filter.Must {
		if expr := buildLanceDBCondition(cond); expr != "" {
			parts = append(parts, expr)
		}
	}

	mustExpr := ""
	if len(parts) > 0 {
		mustExpr = strings.Join(parts, " AND ")
	}

	var shouldParts []string
	for _, cond := range filter.Should {
		if expr := buildLanceDBCondition(cond); expr != "" {
			shouldParts = append(shouldParts, expr)
		}
	}

	shouldExpr := ""
	if len(shouldParts) > 0 {
		shouldExpr = "(" + strings.Join(shouldParts, " OR ") + ")"
	}

	if mustExpr != "" && shouldExpr != "" {
		return mustExpr + " AND " + shouldExpr
	}
	if mustExpr != "" {
		return mustExpr
	}
	return shouldExpr
}

func buildLanceDBCondition(cond Condition) string {
	switch cond.Operator {
	case OpEqual:
		switch v := cond.Value.(type) {
		case string:
			return fmt.Sprintf(`%s = '%s'`, cond.Field, v)
		default:
			return fmt.Sprintf(`%s = %v`, cond.Field, v)
		}

	case OpIn:
		if values, ok := cond.Value.([]string); ok {
			quoted := make([]string, len(values))
			for i, v := range values {
				quoted[i] = fmt.Sprintf("'%s'", v)
			}
			return fmt.Sprintf(`%s IN (%s)`, cond.Field, strings.Join(quoted, ", "))
		}

	case OpRange:
		if rangeVal, ok := cond.Value.(map[string]interface{}); ok {
			var rangeParts []string
			if gte, ok := rangeVal["gte"]; ok {
				rangeParts = append(rangeParts, fmt.Sprintf(`%s >= %v`, cond.Field, gte))
			}
			if lte, ok := rangeVal["lte"]; ok {
				rangeParts = append(rangeParts, fmt.Sprintf(`%s <= %v`, cond.Field, lte))
			}
			return strings.Join(rangeParts, " AND ")
		}
	}
	return ""
}

