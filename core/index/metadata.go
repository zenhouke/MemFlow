package index

import (
	"strings"
	"sync"
	"time"
)

type Metadata struct {
	Entities  []string
	Topic     string
	Timestamp time.Time
	Tags      []string
	Extra     map[string]string
}

type MetadataIndex struct {
	metadataList map[string]Metadata
	entityIndex  map[string][]string
	topicIndex   map[string][]string
	tagIndex     map[string][]string
	timeIndex    map[string]time.Time
	mu           sync.RWMutex
}

func NewMetadataIndex() *MetadataIndex {
	return &MetadataIndex{
		metadataList: make(map[string]Metadata),
		entityIndex:  make(map[string][]string),
		topicIndex:   make(map[string][]string),
		tagIndex:     make(map[string][]string),
		timeIndex:    make(map[string]time.Time),
	}
}

func (idx *MetadataIndex) Add(docID string, meta Metadata) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.metadataList[docID] = meta

	for _, entity := range meta.Entities {
		entity = strings.ToLower(entity)
		idx.entityIndex[entity] = append(idx.entityIndex[entity], docID)
	}

	if meta.Topic != "" {
		topic := strings.ToLower(meta.Topic)
		idx.topicIndex[topic] = append(idx.topicIndex[topic], docID)
	}

	for _, tag := range meta.Tags {
		tag = strings.ToLower(tag)
		idx.tagIndex[tag] = append(idx.tagIndex[tag], docID)
	}

	idx.timeIndex[docID] = meta.Timestamp
}

func (idx *MetadataIndex) Delete(docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	meta, ok := idx.metadataList[docID]
	if !ok {
		return
	}

	for _, entity := range meta.Entities {
		entity = strings.ToLower(entity)
		idx.entityIndex[entity] = idx.removeFromSlice(idx.entityIndex[entity], docID)
		if len(idx.entityIndex[entity]) == 0 {
			delete(idx.entityIndex, entity)
		}
	}

	if meta.Topic != "" {
		topic := strings.ToLower(meta.Topic)
		idx.topicIndex[topic] = idx.removeFromSlice(idx.topicIndex[topic], docID)
		if len(idx.topicIndex[topic]) == 0 {
			delete(idx.topicIndex, topic)
		}
	}

	for _, tag := range meta.Tags {
		tag = strings.ToLower(tag)
		idx.tagIndex[tag] = idx.removeFromSlice(idx.tagIndex[tag], docID)
		if len(idx.tagIndex[tag]) == 0 {
			delete(idx.tagIndex, tag)
		}
	}

	delete(idx.metadataList, docID)
	delete(idx.timeIndex, docID)
}

func (idx *MetadataIndex) removeFromSlice(slice []string, val string) []string {
	for i, v := range slice {
		if v == val {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

type MetadataQuery struct {
	Entities  []string
	Topic     string
	Tags      []string
	TimeStart *time.Time
	TimeEnd   *time.Time
}

func (idx *MetadataIndex) Search(query MetadataQuery) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.metadataList) == 0 {
		return nil
	}

	candidates := make(map[string]int)

	if len(query.Entities) > 0 {
		for _, entity := range query.Entities {
			entity = strings.ToLower(entity)
			if docIDs, ok := idx.entityIndex[entity]; ok {
				for _, docID := range docIDs {
					candidates[docID]++
				}
			}
		}
	}

	if query.Topic != "" {
		topic := strings.ToLower(query.Topic)
		if docIDs, ok := idx.topicIndex[topic]; ok {
			for _, docID := range docIDs {
				candidates[docID] += 2
			}
		}
	}

	if len(query.Tags) > 0 {
		for _, tag := range query.Tags {
			tag = strings.ToLower(tag)
			if docIDs, ok := idx.tagIndex[tag]; ok {
				for _, docID := range docIDs {
					candidates[docID]++
				}
			}
		}
	}

	var results []string
	for docID := range candidates {
		meta := idx.metadataList[docID]

		if query.TimeStart != nil && meta.Timestamp.Before(*query.TimeStart) {
			continue
		}
		if query.TimeEnd != nil && meta.Timestamp.After(*query.TimeEnd) {
			continue
		}

		results = append(results, docID)
	}

	if len(query.Entities) == 0 && query.Topic == "" && len(query.Tags) == 0 {
		for docID := range idx.metadataList {

			meta := idx.metadataList[docID]
			if query.TimeStart != nil && meta.Timestamp.Before(*query.TimeStart) {
				continue
			}
			if query.TimeEnd != nil && meta.Timestamp.After(*query.TimeEnd) {
				continue
			}
			results = append(results, docID)
		}
	}

	return results
}

func (idx *MetadataIndex) GetMetadata(docID string) *Metadata {
	meta, ok := idx.metadataList[docID]
	if !ok {
		return nil
	}
	return &meta
}

func (idx *MetadataIndex) SearchByTimeRange(start, end time.Time) []string {
	var results []string

	for docID, ts := range idx.timeIndex {
		if (ts.Equal(start) || ts.After(start)) &&
			(ts.Equal(end) || ts.Before(end)) {
			results = append(results, docID)
		}
	}

	return results
}

func (idx *MetadataIndex) SearchByEntity(entity string) []string {
	entity = strings.ToLower(entity)
	if docIDs, ok := idx.entityIndex[entity]; ok {
		return docIDs
	}
	return nil
}

func (idx *MetadataIndex) SearchByTopic(topic string) []string {
	topic = strings.ToLower(topic)
	if docIDs, ok := idx.topicIndex[topic]; ok {
		return docIDs
	}
	return nil
}
