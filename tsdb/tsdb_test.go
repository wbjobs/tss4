package tsdb

import (
	"gorilla-tsdb/block"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestBlockCompressDecompress(t *testing.T) {
	b := block.NewBlock()

	now := time.Now().UnixNano()
	for i := 0; i < 64; i++ {
		point := block.DataPoint{
			Timestamp: now + int64(i)*1000000000,
			Value:     float64(i) * 1.5,
		}
		if !b.Add(point) {
			t.Fatalf("failed to add point %d", i)
		}
	}

	if !b.Full {
		t.Fatal("block should be full")
	}

	data, err := b.Compress()
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	originalSize := 64 * (8 + 8)
	t.Logf("Original size: %d bytes, compressed size: %d bytes, ratio: %.2f%%",
		originalSize, len(data), float64(len(data))/float64(originalSize)*100)

	b2, err := block.Decompress(data)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	if len(b2.Points) != 64 {
		t.Fatalf("expected 64 points, got %d", len(b2.Points))
	}

	for i := 0; i < 64; i++ {
		if b2.Points[i].Timestamp != b.Points[i].Timestamp {
			t.Errorf("point %d timestamp mismatch: expected %d, got %d",
				i, b.Points[i].Timestamp, b2.Points[i].Timestamp)
		}
		if b2.Points[i].Value != b.Points[i].Value {
			t.Errorf("point %d value mismatch: expected %v, got %v",
				i, b.Points[i].Value, b2.Points[i].Value)
		}
	}
}

func TestBlockPersistLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persister, err := block.NewPersister(tmpDir)
	if err != nil {
		t.Fatalf("create persister: %v", err)
	}

	b := block.NewBlock()
	now := time.Now().UnixNano()
	for i := 0; i < 64; i++ {
		b.Add(block.DataPoint{
			Timestamp: now + int64(i)*1000000000,
			Value:     float64(i) * 1.5,
		})
	}

	filePath, err := persister.SaveBlock(b)
	if err != nil {
		t.Fatalf("save block: %v", err)
	}

	t.Logf("Block saved to: %s", filePath)

	b2, err := persister.LoadBlock(filePath)
	if err != nil {
		t.Fatalf("load block: %v", err)
	}

	if len(b2.Points) != 64 {
		t.Fatalf("expected 64 points, got %d", len(b2.Points))
	}

	for i := 0; i < 64; i++ {
		if b2.Points[i].Timestamp != b.Points[i].Timestamp {
			t.Errorf("point %d timestamp mismatch", i)
		}
		if b2.Points[i].Value != b.Points[i].Value {
			t.Errorf("point %d value mismatch", i)
		}
	}
}

func TestTSDBWriteQuery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("create TSDB: %v", err)
	}
	defer db.Close()

	now := time.Now().UnixNano()
	numPoints := 150

	for i := 0; i < numPoints; i++ {
		err := db.Write(block.DataPoint{
			Timestamp: now + int64(i)*1000000000,
			Value:     float64(i) * 1.5,
		})
		if err != nil {
			t.Fatalf("write point %d: %v", i, err)
		}
	}

	entries := db.Index().Entries()
	t.Logf("Persisted blocks: %d", len(entries))
	t.Logf("Active block points: %d", len(db.ActiveBlock().Points))

	expectedBlocks := (numPoints - 1) / 64
	if len(entries) != expectedBlocks {
		t.Errorf("expected %d persisted blocks, got %d", expectedBlocks, len(entries))
	}

	expectedActive := numPoints - expectedBlocks*64
	if len(db.ActiveBlock().Points) != expectedActive {
		t.Errorf("expected %d active points, got %d", expectedActive, len(db.ActiveBlock().Points))
	}

	startTime := now + 10*1000000000
	endTime := now + 50*1000000000

	points, err := db.Query(startTime, endTime)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Query returned %d points", len(points))

	if len(points) != 41 {
		t.Errorf("expected 41 points, got %d", len(points))
	}

	for i, p := range points {
		expectedTS := now + int64(i+10)*1000000000
		expectedVal := float64(i+10) * 1.5
		if p.Timestamp != expectedTS {
			t.Errorf("point %d timestamp: expected %d, got %d", i, expectedTS, p.Timestamp)
		}
		if p.Value != expectedVal {
			t.Errorf("point %d value: expected %v, got %v", i, expectedVal, p.Value)
		}
	}
}

func TestTSDBFlush(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("create TSDB: %v", err)
	}

	now := time.Now().UnixNano()
	for i := 0; i < 10; i++ {
		db.Write(block.DataPoint{
			Timestamp: now + int64(i)*1000000000,
			Value:     float64(i),
		})
	}

	if len(db.ActiveBlock().Points) != 10 {
		t.Fatalf("expected 10 active points")
	}

	if len(db.Index().Entries()) != 0 {
		t.Fatalf("expected 0 persisted blocks before flush")
	}

	if err := db.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	if len(db.ActiveBlock().Points) != 0 {
		t.Fatalf("expected 0 active points after flush")
	}

	if len(db.Index().Entries()) != 1 {
		t.Fatalf("expected 1 persisted block after flush, got %d", len(db.Index().Entries()))
	}

	points, err := db.Query(now, now+10*1000000000)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(points) != 10 {
		t.Errorf("expected 10 points from query, got %d", len(points))
	}

	db.Close()

	db2, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("reopen TSDB: %v", err)
	}
	defer db2.Close()

	if len(db2.Index().Entries()) != 1 {
		t.Errorf("expected 1 block after reload, got %d", len(db2.Index().Entries()))
	}

	points2, err := db2.Query(now, now+10*1000000000)
	if err != nil {
		t.Fatalf("query after reload failed: %v", err)
	}

	if len(points2) != 10 {
		t.Errorf("expected 10 points after reload, got %d", len(points2))
	}
}

func TestOutOfOrderWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("create TSDB: %v", err)
	}
	defer db.Close()

	now := time.Now().UnixNano()

	points := []block.DataPoint{
		{Timestamp: now + 5*1000000000, Value: 5.0},
		{Timestamp: now + 2*1000000000, Value: 2.0},
		{Timestamp: now + 8*1000000000, Value: 8.0},
		{Timestamp: now + 1*1000000000, Value: 1.0},
		{Timestamp: now + 3*1000000000, Value: 3.0},
		{Timestamp: now + 7*1000000000, Value: 7.0},
		{Timestamp: now + 4*1000000000, Value: 4.0},
		{Timestamp: now + 6*1000000000, Value: 6.0},
	}

	for _, p := range points {
		if err := db.Write(p); err != nil {
			t.Fatalf("write point: %v", err)
		}
	}

	activeBlock := db.ActiveBlock()
	if len(activeBlock.Points) != 8 {
		t.Fatalf("expected 8 points, got %d", len(activeBlock.Points))
	}

	for i := 1; i < len(activeBlock.Points); i++ {
		if activeBlock.Points[i].Timestamp <= activeBlock.Points[i-1].Timestamp {
			t.Errorf("points not sorted at index %d: prev=%d, curr=%d",
				i, activeBlock.Points[i-1].Timestamp, activeBlock.Points[i].Timestamp)
		}
	}

	for i, p := range activeBlock.Points {
		expectedTS := now + int64(i+1)*1000000000
		expectedVal := float64(i + 1)
		if p.Timestamp != expectedTS {
			t.Errorf("point %d timestamp: expected %d, got %d", i, expectedTS, p.Timestamp)
		}
		if p.Value != expectedVal {
			t.Errorf("point %d value: expected %v, got %v", i, expectedVal, p.Value)
		}
	}

	queryResult, err := db.Query(now, now+10*1000000000)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(queryResult) != 8 {
		t.Errorf("query expected 8 points, got %d", len(queryResult))
	}

	for i, p := range queryResult {
		expectedTS := now + int64(i+1)*1000000000
		if p.Timestamp != expectedTS {
			t.Errorf("query point %d timestamp: expected %d, got %d", i, expectedTS, p.Timestamp)
		}
	}

	t.Log("Out-of-order write test passed")
}

func TestDuplicateTimestamp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("create TSDB: %v", err)
	}
	defer db.Close()

	now := time.Now().UnixNano()

	db.Write(block.DataPoint{Timestamp: now, Value: 1.0})
	db.Write(block.DataPoint{Timestamp: now, Value: 2.0})
	db.Write(block.DataPoint{Timestamp: now, Value: 3.0})

	activeBlock := db.ActiveBlock()
	if len(activeBlock.Points) != 1 {
		t.Fatalf("expected 1 point after duplicates, got %d", len(activeBlock.Points))
	}

	if activeBlock.Points[0].Value != 3.0 {
		t.Errorf("expected value 3.0 (last write wins), got %v", activeBlock.Points[0].Value)
	}

	queryResult, err := db.Query(now, now)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(queryResult) != 1 {
		t.Errorf("query expected 1 point, got %d", len(queryResult))
	}

	if queryResult[0].Value != 3.0 {
		t.Errorf("query expected value 3.0, got %v", queryResult[0].Value)
	}

	t.Log("Duplicate timestamp test passed")
}

func TestQueryDeduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("create TSDB: %v", err)
	}
	defer db.Close()

	now := time.Now().UnixNano()

	for i := 0; i < 70; i++ {
		db.Write(block.DataPoint{
			Timestamp: now + int64(i)*1000000000,
			Value:     float64(i),
		})
	}

	if err := db.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	db.Write(block.DataPoint{
		Timestamp: now + 30*1000000000,
		Value:     999.0,
	})

	queryResult, err := db.Query(now, now+100*1000000000)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(queryResult) != 70 {
		t.Errorf("expected 70 unique points, got %d", len(queryResult))
	}

	seen := make(map[int64]bool)
	for _, p := range queryResult {
		if seen[p.Timestamp] {
			t.Errorf("duplicate timestamp found: %d", p.Timestamp)
		}
		seen[p.Timestamp] = true
	}

	for i, p := range queryResult {
		if i > 0 && p.Timestamp <= queryResult[i-1].Timestamp {
			t.Errorf("points not sorted at index %d", i)
		}
	}

	t.Log("Query deduplication test passed")
}

func TestCompressionRatio(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tsdb-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewTSDB(tmpDir)
	if err != nil {
		t.Fatalf("create TSDB: %v", err)
	}
	defer db.Close()

	now := time.Now().UnixNano()

	t.Run("constant_interval_constant_value", func(t *testing.T) {
		for i := 0; i < 64; i++ {
			db.Write(block.DataPoint{
				Timestamp: now + int64(i)*1000000000,
				Value:     42.0,
			})
		}
		db.Flush()

		entries := db.Index().Entries()
		if len(entries) != 1 {
			t.Fatalf("expected 1 block")
		}

		data, err := os.ReadFile(entries[0].FilePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		originalSize := 64 * (8 + 8)
		ratio := float64(len(data)) / float64(originalSize) * 100
		t.Logf("Constant data - Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
			originalSize, len(data), ratio)
	})

	t.Run("constant_interval_slowly_changing", func(t *testing.T) {
		db2, _ := NewTSDB(tmpDir + "_2")
		defer os.RemoveAll(tmpDir + "_2")
		defer db2.Close()

		for i := 0; i < 64; i++ {
			db2.Write(block.DataPoint{
				Timestamp: now + int64(i)*1000000000,
				Value:     100.0 + float64(i)*0.01,
			})
		}
		db2.Flush()

		entries := db2.Index().Entries()
		data, _ := os.ReadFile(entries[0].FilePath)

		originalSize := 64 * (8 + 8)
		ratio := float64(len(data)) / float64(originalSize) * 100
		t.Logf("Slowly changing - Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
			originalSize, len(data), ratio)
	})

	t.Run("random_data", func(t *testing.T) {
		db3, _ := NewTSDB(tmpDir + "_3")
		defer os.RemoveAll(tmpDir + "_3")
		defer db3.Close()

		ts := now
		for i := 0; i < 64; i++ {
			ts += rand.Int63n(5000000000) + 1000000000
			db3.Write(block.DataPoint{
				Timestamp: ts,
				Value:     rand.Float64() * 1000,
			})
		}
		db3.Flush()

		entries := db3.Index().Entries()
		data, _ := os.ReadFile(entries[0].FilePath)

		originalSize := 64 * (8 + 8)
		ratio := float64(len(data)) / float64(originalSize) * 100
		t.Logf("Random data - Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
			originalSize, len(data), ratio)
	})
}
