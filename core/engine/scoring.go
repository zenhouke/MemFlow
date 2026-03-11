package engine

import (
	"math"
	"memflow/core/utils"
	"time"
)

func (m *MemoryEngine) score(now time.Time, q []float64, mem *MemoryItem, decay float64) float64 {
	relevance := utils.Cosine(q, mem.Embedding)

	hours := now.Sub(mem.LastAccessedAt).Hours()
	recency := math.Exp(-decay * hours)

	return m.config.Alpha*relevance +
		m.config.Beta*recency +
		m.config.Gamma*mem.Importance
}
