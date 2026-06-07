package index

import (
	"encoding/binary"
	"fmt"
	"gorilla-tsdb/block"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
)

type IndexEntry struct {
	MinTime  int64
	MaxTime  int64
	FilePath string
}

type Index struct {
	mu        sync.RWMutex
	entries   []IndexEntry
	persister *block.Persister
}

var blockFileRegex = regexp.MustCompile(`^block_(-?\d+)_(-?\d+)\.bin$`)

func NewIndex(dataDir string) (*Index, error) {
	persister, err := block.NewPersister(dataDir)
	if err != nil {
		return nil, fmt.Errorf("create persister: %w", err)
	}

	idx := &Index{
		entries:   make([]IndexEntry, 0),
		persister: persister,
	}

	if err := idx.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("load index from disk: %w", err)
	}

	return idx, nil
}

func (idx *Index) loadFromDisk() error {
	dataDir := idx.persister.DataDir()

	files, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := blockFileRegex.FindStringSubmatch(file.Name())
		if len(matches) != 3 {
			continue
		}

		minTime, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			continue
		}

		maxTime, err := strconv.ParseInt(matches[2], 10, 64)
		if err != nil {
			continue
		}

		filePath := filepath.Join(dataDir, file.Name())

		count, err := readBlockCount(filePath)
		if err != nil || count == 0 {
			continue
		}

		idx.entries = append(idx.entries, IndexEntry{
			MinTime:  minTime,
			MaxTime:  maxTime,
			FilePath: filePath,
		})
	}

	sort.Slice(idx.entries, func(i, j int) bool {
		return idx.entries[i].MinTime < idx.entries[j].MinTime
	})

	return nil
}

func readBlockCount(filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	header := make([]byte, 18)
	if _, err := f.Read(header); err != nil {
		return 0, err
	}

	count := int(binary.LittleEndian.Uint16(header[16:18]))
	return count, nil
}

func (idx *Index) AddBlock(b *block.Block) (string, error) {
	filePath, err := idx.persister.SaveBlock(b)
	if err != nil {
		return "", err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	entry := IndexEntry{
		MinTime:  b.MinTime,
		MaxTime:  b.MaxTime,
		FilePath: filePath,
	}

	idx.entries = append(idx.entries, entry)
	sort.Slice(idx.entries, func(i, j int) bool {
		return idx.entries[i].MinTime < idx.entries[j].MinTime
	})

	return filePath, nil
}

func (idx *Index) Query(startTime, endTime int64) ([]block.DataPoint, error) {
	idx.mu.RLock()
	candidates := make([]IndexEntry, 0)
	for _, entry := range idx.entries {
		if entry.MaxTime >= startTime && entry.MinTime <= endTime {
			candidates = append(candidates, entry)
		}
	}
	idx.mu.RUnlock()

	result := make([]block.DataPoint, 0)

	for _, entry := range candidates {
		b, err := idx.persister.LoadBlock(entry.FilePath)
		if err != nil {
			return nil, fmt.Errorf("load block %s: %w", entry.FilePath, err)
		}

		for _, p := range b.Points {
			if p.Timestamp >= startTime && p.Timestamp <= endTime {
				result = append(result, p)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result, nil
}

func (idx *Index) Entries() []IndexEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]IndexEntry, len(idx.entries))
	copy(result, idx.entries)
	return result
}

func (idx *Index) Persister() *block.Persister {
	return idx.persister
}
