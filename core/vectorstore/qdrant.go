package vectorstore

import (
	"context"
	"fmt"
	"simplemem/core/config"

	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	Register("qdrant", func(cfg config.VectorStoreConfig) (VectorStore, error) {
		return NewQdrant(cfg)
	})
}
func NewQdrant(cfg config.VectorStoreConfig) (*QdrantStore, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 6334 // Qdrant gRPC 默认端口
	}

	addr := cfg.Address()

	// 建立 gRPC 连接
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant at %s: %w", addr, err)
	}

	store := &QdrantStore{
		conn:           conn,
		pointsClient:   pb.NewPointsClient(conn),
		collectClient:  pb.NewCollectionsClient(conn),
		collectionName: cfg.CollectionName,
		dimension:      cfg.Dimension,
	}

	// 确保集合存在
	if err := store.ensureCollection(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	return store, nil
}
type QdrantStore struct {
	conn           *grpc.ClientConn
	pointsClient   pb.PointsClient
	collectClient  pb.CollectionsClient
	collectionName string
	dimension      int
}



func (q *QdrantStore) ensureCollection(ctx context.Context) error {
	// 检查集合是否存在
	resp, err := q.collectClient.List(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	for _, col := range resp.GetCollections() {
		if col.GetName() == q.collectionName {
			return nil // 集合已存在
		}
	}

	// 创建集合
	dim := uint64(q.dimension)
	_, err = q.collectClient.Create(ctx, &pb.CreateCollection{
		CollectionName: q.collectionName,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     dim,
					Distance: pb.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", q.collectionName, err)
	}

	return nil
}

func (q *QdrantStore) Add(ctx context.Context, vectors []VectorRecord) error {
	return q.upsertPoints(ctx, vectors)
}

func (q *QdrantStore) Upsert(ctx context.Context, vectors []VectorRecord) error {
	return q.upsertPoints(ctx, vectors)
}

func (q *QdrantStore) upsertPoints(ctx context.Context, vectors []VectorRecord) error {
	points := make([]*pb.PointStruct, 0, len(vectors))

	for _, v := range vectors {
		payload := convertToQdrantPayload(v.Payload)
		point := &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{
					Uuid: v.ID,
				},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{
						Data: v.Vector,
					},
				},
			},
			Payload: payload,
		}
		points = append(points, point)
	}

	waitUpsert := true
	_, err := q.pointsClient.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: q.collectionName,
		Wait:           &waitUpsert,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	return nil
}

func (q *QdrantStore) Search(ctx context.Context, query []float32, topK int, filter *Filter) ([]SearchResult, error) {
	searchReq := &pb.SearchPoints{
		CollectionName: q.collectionName,
		Vector:         query,
		Limit:          uint64(topK),
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
	}

	// 构建过滤器
	if filter != nil {
		searchReq.Filter = buildQdrantFilter(filter)
	}

	resp, err := q.pointsClient.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	results := make([]SearchResult, 0, len(resp.GetResult()))
	for _, r := range resp.GetResult() {
		result := SearchResult{
			ID:      extractPointID(r.GetId()),
			Score:   r.GetScore(),
			Payload: convertFromQdrantPayload(r.GetPayload()),
		}
		results = append(results, result)
	}

	return results, nil
}

func (q *QdrantStore) Delete(ctx context.Context, ids []string) error {
	pointIDs := make([]*pb.PointId, 0, len(ids))
	for _, id := range ids {
		pointIDs = append(pointIDs, &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: id,
			},
		})
	}

	waitDelete := true
	_, err := q.pointsClient.Delete(ctx, &pb.DeletePoints{
		CollectionName: q.collectionName,
		Wait:           &waitDelete,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: pointIDs,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}

	return nil
}

func (q *QdrantStore) Close() error {
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}

// ========== Qdrant 过滤器构建 ==========

func buildQdrantFilter(filter *Filter) *pb.Filter {
	if filter == nil {
		return nil
	}

	f := &pb.Filter{}

	for _, cond := range filter.Must {
		clause := buildQdrantCondition(cond)
		if clause != nil {
			f.Must = append(f.Must, clause)
		}
	}

	for _, cond := range filter.Should {
		clause := buildQdrantCondition(cond)
		if clause != nil {
			f.Should = append(f.Should, clause)
		}
	}

	return f
}

func buildQdrantCondition(cond Condition) *pb.Condition {
	switch cond.Operator {
	case OpEqual:
		return &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key:   cond.Field,
					Match: buildQdrantMatch(cond.Value),
				},
			},
		}

	case OpIn:
		if values, ok := cond.Value.([]string); ok {
			return &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: cond.Field,
						Match: &pb.Match{
							MatchValue: &pb.Match_Keywords{
								Keywords: &pb.RepeatedStrings{
									Strings: values,
								},
							},
						},
					},
				},
			}
		}
		if values, ok := cond.Value.([]int64); ok {
			return &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: cond.Field,
						Match: &pb.Match{
							MatchValue: &pb.Match_Integers{
								Integers: &pb.RepeatedIntegers{
									Integers: values,
								},
							},
						},
					},
				},
			}
		}

	case OpRange:
		if rangeVal, ok := cond.Value.(map[string]interface{}); ok {
			r := &pb.Range{}
			if gte, ok := rangeVal["gte"]; ok {
				if v, ok := toFloat64(gte); ok {
					r.Gte = &v
				}
			}
			if lte, ok := rangeVal["lte"]; ok {
				if v, ok := toFloat64(lte); ok {
					r.Lte = &v
				}
			}
			if gt, ok := rangeVal["gt"]; ok {
				if v, ok := toFloat64(gt); ok {
					r.Gt = &v
				}
			}
			if lt, ok := rangeVal["lt"]; ok {
				if v, ok := toFloat64(lt); ok {
					r.Lt = &v
				}
			}
			return &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key:   cond.Field,
						Range: r,
					},
				},
			}
		}
	}

	return nil
}

func buildQdrantMatch(value interface{}) *pb.Match {
	switch v := value.(type) {
	case string:
		return &pb.Match{
			MatchValue: &pb.Match_Keyword{Keyword: v},
		}
	case int:
		return &pb.Match{
			MatchValue: &pb.Match_Integer{Integer: int64(v)},
		}
	case int64:
		return &pb.Match{
			MatchValue: &pb.Match_Integer{Integer: v},
		}
	case bool:
		return &pb.Match{
			MatchValue: &pb.Match_Boolean{Boolean: v},
		}
	default:
		return &pb.Match{
			MatchValue: &pb.Match_Keyword{Keyword: fmt.Sprintf("%v", v)},
		}
	}
}

// ========== 数据转换辅助函数 ==========

func convertToQdrantPayload(payload map[string]interface{}) map[string]*pb.Value {
	if payload == nil {
		return nil
	}

	result := make(map[string]*pb.Value, len(payload))
	for key, val := range payload {
		result[key] = toQdrantValue(val)
	}
	return result
}

func toQdrantValue(val interface{}) *pb.Value {
	switch v := val.(type) {
	case string:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: v}}
	case int:
		return &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: int64(v)}}
	case int64:
		return &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: v}}
	case float64:
		return &pb.Value{Kind: &pb.Value_DoubleValue{DoubleValue: v}}
	case float32:
		return &pb.Value{Kind: &pb.Value_DoubleValue{DoubleValue: float64(v)}}
	case bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: v}}
	default:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: fmt.Sprintf("%v", v)}}
	}
}

func convertFromQdrantPayload(payload map[string]*pb.Value) map[string]interface{} {
	if payload == nil {
		return nil
	}

	result := make(map[string]interface{}, len(payload))
	for key, val := range payload {
		result[key] = fromQdrantValue(val)
	}
	return result
}

func fromQdrantValue(val *pb.Value) interface{} {
	if val == nil {
		return nil
	}

	switch v := val.Kind.(type) {
	case *pb.Value_StringValue:
		return v.StringValue
	case *pb.Value_IntegerValue:
		return v.IntegerValue
	case *pb.Value_DoubleValue:
		return v.DoubleValue
	case *pb.Value_BoolValue:
		return v.BoolValue
	default:
		return nil
	}
}

func extractPointID(id *pb.PointId) string {
	if id == nil {
		return ""
	}
	switch v := id.PointIdOptions.(type) {
	case *pb.PointId_Uuid:
		return v.Uuid
	case *pb.PointId_Num:
		return fmt.Sprintf("%d", v.Num)
	default:
		return ""
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}