package index

import (
	"math"
	"strings"
	"sync"
)

type BM25Index struct {
	documents  map[string]string
	docFreq    map[string]int
	docLengths map[string]int
	avgDocLen  float64
	totalLen   int // 新增：记录总长度以便增量计算
	k1         float64
	b          float64
	docCount   int
	mu         sync.RWMutex
}

func NewBM25Index() *BM25Index {
	return &BM25Index{
		documents:  make(map[string]string),
		docFreq:    make(map[string]int),
		docLengths: make(map[string]int),
		k1:         1.5,
		b:          0.75,
	}
}

func (idx *BM25Index) Add(docID string, doc string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if oldDoc, exists := idx.documents[docID]; exists {
		oldTerms := idx.tokenize(oldDoc)
		idx.totalLen -= len(oldTerms)

		seenOld := make(map[string]bool)
		for _, term := range oldTerms {
			if !seenOld[term] {
				idx.docFreq[term]--
				if idx.docFreq[term] <= 0 {
					delete(idx.docFreq, term)
				}
				seenOld[term] = true
			}
		}
	} else {
		idx.docCount++
	}

	idx.documents[docID] = doc

	terms := idx.tokenize(doc)
	idx.docLengths[docID] = len(terms)
	idx.totalLen += len(terms) // 增量更新

	seen := make(map[string]bool)
	for _, term := range terms {
		if !seen[term] {
			idx.docFreq[term]++
			seen[term] = true
		}
	}
	idx.recalculateAvgDocLen()
}

func (idx *BM25Index) Delete(docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	doc, ok := idx.documents[docID]
	if !ok {
		return
	}

	terms := idx.tokenize(doc)
	idx.totalLen -= len(terms) // 增量更新

	seen := make(map[string]bool)
	for _, term := range terms {
		if !seen[term] {
			idx.docFreq[term]--
			if idx.docFreq[term] <= 0 {
				delete(idx.docFreq, term)
			}
			seen[term] = true
		}
	}

	delete(idx.documents, docID)
	delete(idx.docLengths, docID)
	idx.docCount--

	idx.recalculateAvgDocLen()
}

func (idx *BM25Index) recalculateAvgDocLen() {
	if idx.docCount > 0 {
		idx.avgDocLen = float64(idx.totalLen) / float64(idx.docCount)
	} else {
		idx.avgDocLen = 0
	}
}

func (idx *BM25Index) Search(query string, topK int) []ScoredDoc {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.documents) == 0 {
		return nil
	}

	queryTerms := idx.tokenize(query)
	scores := make([]ScoredDoc, 0, len(idx.documents))

	for docID, doc := range idx.documents {
		score := idx.calculateBM25(queryTerms, doc, docID)
		if score > 0 {
			scores = append(scores, ScoredDoc{
				DocID: docID,
				Score: score,
			})
		}
	}

	sortScoredDocs(scores)

	if topK > 0 && topK < len(scores) {
		return scores[:topK]
	}
	return scores
}

func (idx *BM25Index) calculateBM25(queryTerms []string, doc string, docID string) float64 {
	docTerms := idx.tokenize(doc)
	docLen := float64(idx.docLengths[docID])

	termFreq := make(map[string]int)
	for _, term := range docTerms {
		termFreq[term]++
	}

	score := 0.0
	N := float64(idx.docCount)

	for _, qTerm := range queryTerms {

		tf := float64(termFreq[qTerm])
		if tf == 0 {
			continue
		}

		df := float64(idx.docFreq[qTerm])
		idf := math.Log((N - df + 0.5) / (df + 0.5))

		numerator := tf * (idx.k1 + 1)
		denominator := tf + idx.k1*(1-idx.b+idx.b*docLen/idx.avgDocLen)

		score += idf * (numerator / denominator)
	}

	return score
}

func (idx *BM25Index) tokenize(text string) []string {

	text = strings.ToLower(text)

	replacer := strings.NewReplacer(
		",", " ", ".", " ", "!", " ", "?", " ",
		";", " ", ":", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "{", " ", "}", " ",
		"\"", " ", "'", " ",
	)
	text = replacer.Replace(text)

	words := strings.Fields(text)

	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true,
		"at": true, "be": true, "by": true, "for": true, "from": true,
		"has": true, "he": true, "in": true, "is": true, "it": true,
		"its": true, "of": true, "on": true, "that": true, "the": true,
		"to": true, "was": true, "will": true, "with": true,
	}

	var filtered []string
	for _, word := range words {
		if len(word) > 1 && !stopWords[word] {
			filtered = append(filtered, word)
		}
	}

	return filtered
}

type ScoredDoc struct {
	DocID string
	Score float64
}

func sortScoredDocs(docs []ScoredDoc) {

	for i := 0; i < len(docs); i++ {
		for j := i + 1; j < len(docs); j++ {
			if docs[j].Score > docs[i].Score {
				docs[i], docs[j] = docs[j], docs[i]
			}
		}
	}
}
