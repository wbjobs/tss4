package tsdb

import (
	"fmt"
	"gorilla-tsdb/block"
	"gorilla-tsdb/downsample"
	"gorilla-tsdb/index"
	"path/filepath"
	"sync"
)

type MultiTSDB struct {
	mu       sync.RWMutex
	dbs      map[string]*TSDB
	dataDir  string
	labels   map[string]map[string]string
}

func NewMultiTSDB(dataDir string) (*MultiTSDB, error) {
	mdb := &MultiTSDB{
		dbs:     make(map[string]*TSDB),
		dataDir: dataDir,
		labels:  make(map[string]map[string]string),
	}
	return mdb, nil
}

func (m *MultiTSDB) getOrCreateDB(seriesKey string, labels map[string]string) (*TSDB, error) {
	m.mu.RLock()
	db, exists := m.dbs[seriesKey]
	m.mu.RUnlock()

	if exists {
		return db, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if db, exists := m.dbs[seriesKey]; exists {
		return db, nil
	}

	seriesDir := filepath.Join(m.dataDir, "series", seriesKey)
	db, err := NewTSDB(seriesDir)
	if err != nil {
		return nil, fmt.Errorf("create TSDB for series %s: %w", seriesKey, err)
	}

	m.dbs[seriesKey] = db
	m.labels[seriesKey] = labels
	return db, nil
}

func (m *MultiTSDB) Write(seriesKey string, labels map[string]string, point block.DataPoint) error {
	db, err := m.getOrCreateDB(seriesKey, labels)
	if err != nil {
		return err
	}
	return db.Write(point)
}

func (m *MultiTSDB) Query(seriesKey string, startTime, endTime int64, downsampleThreshold int) ([]block.DataPoint, error) {
	m.mu.RLock()
	db, exists := m.dbs[seriesKey]
	m.mu.RUnlock()

	if !exists {
		return []block.DataPoint{}, nil
	}

	points, err := db.Query(startTime, endTime)
	if err != nil {
		return nil, err
	}

	if downsampleThreshold > 0 && len(points) > downsampleThreshold {
		points = downsample.LTTB(points, downsampleThreshold)
	}

	return points, nil
}

func (m *MultiTSDB) QueryWithDownsample(seriesKey string, startTime, endTime int64, threshold int) ([]block.DataPoint, error) {
	return m.Query(seriesKey, startTime, endTime, threshold)
}

func (m *MultiTSDB) Flush() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var firstErr error
	for _, db := range m.dbs {
		if err := db.Flush(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiTSDB) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var firstErr error
	for _, db := range m.dbs {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiTSDB) GetSeriesKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.dbs))
	for k := range m.dbs {
		keys = append(keys, k)
	}
	return keys
}

func (m *MultiTSDB) GetLabels(seriesKey string) (map[string]string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	labels, exists := m.labels[seriesKey]
	if !exists {
		return nil, false
	}

	result := make(map[string]string, len(labels))
	for k, v := range labels {
		result[k] = v
	}
	return result, true
}

func (m *MultiTSDB) FindSeriesByMatcher(matchers map[string]string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []string

	for key, labels := range m.labels {
		matched := true
		for mk, mv := range matchers {
			if v, ok := labels[mk]; !ok || v != mv {
				matched = false
				break
			}
		}
		if matched {
			result = append(result, key)
		}
	}

	return result
}

func (m *MultiTSDB) Index(seriesKey string) (*index.Index, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db, exists := m.dbs[seriesKey]
	if !exists {
		return nil, false
	}
	return db.Index(), true
}
