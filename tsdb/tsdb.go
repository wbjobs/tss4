package tsdb

import (
	"fmt"
	"gorilla-tsdb/block"
	"gorilla-tsdb/index"
	"sync"
)

type TSDB struct {
	mu          sync.Mutex
	activeBlock *block.Block
	index       *index.Index
}

func NewTSDB(dataDir string) (*TSDB, error) {
	idx, err := index.NewIndex(dataDir)
	if err != nil {
		return nil, fmt.Errorf("create index: %w", err)
	}

	db := &TSDB{
		activeBlock: block.NewBlock(),
		index:       idx,
	}

	return db, nil
}

func (db *TSDB) Write(point block.DataPoint) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if !db.activeBlock.Add(point) {
		if _, err := db.index.AddBlock(db.activeBlock); err != nil {
			return fmt.Errorf("persist block: %w", err)
		}

		db.activeBlock = block.NewBlock()
		if !db.activeBlock.Add(point) {
			return fmt.Errorf("failed to add point to new block")
		}
	}

	return nil
}

func (db *TSDB) WriteBatch(points []block.DataPoint) error {
	for _, p := range points {
		if err := db.Write(p); err != nil {
			return err
		}
	}
	return nil
}

func (db *TSDB) Query(startTime, endTime int64) ([]block.DataPoint, error) {
	result, err := db.index.Query(startTime, endTime)
	if err != nil {
		return nil, err
	}

	db.mu.Lock()
	activeBlock := db.activeBlock
	db.mu.Unlock()

	if activeBlock != nil && len(activeBlock.Points) > 0 {
		if activeBlock.MaxTime >= startTime && activeBlock.MinTime <= endTime {
			for _, p := range activeBlock.Points {
				if p.Timestamp >= startTime && p.Timestamp <= endTime {
					result = append(result, p)
				}
			}
		}
	}

	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j-1].Timestamp > result[j].Timestamp; j-- {
			result[j-1], result[j] = result[j], result[j-1]
		}
	}

	return result, nil
}

func (db *TSDB) Flush() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.activeBlock != nil && len(db.activeBlock.Points) > 0 {
		if _, err := db.index.AddBlock(db.activeBlock); err != nil {
			return fmt.Errorf("persist active block: %w", err)
		}
		db.activeBlock = block.NewBlock()
	}

	return nil
}

func (db *TSDB) Close() error {
	return db.Flush()
}

func (db *TSDB) Index() *index.Index {
	return db.index
}

func (db *TSDB) ActiveBlock() *block.Block {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.activeBlock
}
