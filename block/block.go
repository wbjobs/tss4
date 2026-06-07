package block

import (
	"encoding/binary"
	"fmt"
	"gorilla-tsdb/compression"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const BlockSize = 64

type DataPoint struct {
	Timestamp int64
	Value     float64
}

type Block struct {
	Points     []DataPoint
	MinTime    int64
	MaxTime    int64
	Full       bool
	compressed []byte
}

func NewBlock() *Block {
	return &Block{
		Points:  make([]DataPoint, 0, BlockSize),
		MinTime: math.MaxInt64,
		MaxTime: math.MinInt64,
	}
}

func (b *Block) Add(point DataPoint) bool {
	if b.Full {
		return false
	}

	idx := sort.Search(len(b.Points), func(i int) bool {
		return b.Points[i].Timestamp >= point.Timestamp
	})

	if idx < len(b.Points) && b.Points[idx].Timestamp == point.Timestamp {
		b.Points[idx].Value = point.Value
		b.compressed = nil
		return true
	}

	if idx == len(b.Points) {
		b.Points = append(b.Points, point)
	} else {
		b.Points = append(b.Points, DataPoint{})
		copy(b.Points[idx+1:], b.Points[idx:])
		b.Points[idx] = point
	}

	b.compressed = nil

	if point.Timestamp < b.MinTime {
		b.MinTime = point.Timestamp
	}
	if point.Timestamp > b.MaxTime {
		b.MaxTime = point.Timestamp
	}
	if len(b.Points) >= BlockSize {
		b.Full = true
	}
	return true
}

func (b *Block) Sort() {
	sort.Slice(b.Points, func(i, j int) bool {
		return b.Points[i].Timestamp < b.Points[j].Timestamp
	})
	b.compressed = nil
}

func (b *Block) Compress() ([]byte, error) {
	if b.compressed != nil {
		return b.compressed, nil
	}

	b.Sort()

	w := compression.NewBitWriter()

	tsEncoder := compression.NewTimestampEncoder(w)
	xorEncoder := compression.NewXOREncoder(w)

	for _, p := range b.Points {
		tsEncoder.Encode(p.Timestamp)
		xorEncoder.Encode(p.Value)
	}

	compressedData := w.Bytes()

	header := make([]byte, 18)
	binary.LittleEndian.PutUint64(header[0:8], uint64(b.MinTime))
	binary.LittleEndian.PutUint64(header[8:16], uint64(b.MaxTime))
	binary.LittleEndian.PutUint16(header[16:18], uint16(len(b.Points)))

	result := make([]byte, 0, len(header)+len(compressedData))
	result = append(result, header...)
	result = append(result, compressedData...)

	b.compressed = result
	return result, nil
}

func Decompress(data []byte) (*Block, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("block data too short: %d bytes", len(data))
	}

	minTime := int64(binary.LittleEndian.Uint64(data[0:8]))
	maxTime := int64(binary.LittleEndian.Uint64(data[8:16]))
	count := int(binary.LittleEndian.Uint16(data[16:18]))

	compressedData := data[18:]

	r := compression.NewBitReader(compressedData)

	tsDecoder := compression.NewTimestampDecoder(r)
	xorDecoder := compression.NewXORDecoder(r)

	b := NewBlock()
	b.MinTime = minTime
	b.MaxTime = maxTime

	for i := 0; i < count; i++ {
		ts, err := tsDecoder.Decode()
		if err != nil {
			return nil, fmt.Errorf("decode timestamp %d: %w", i, err)
		}

		val, err := xorDecoder.Decode()
		if err != nil {
			return nil, fmt.Errorf("decode value %d: %w", i, err)
		}

		b.Points = append(b.Points, DataPoint{Timestamp: ts, Value: val})
	}

	b.Full = len(b.Points) >= BlockSize
	return b, nil
}

func ReadBlockFromFile(filePath string) (*Block, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read block file %s: %w", filePath, err)
	}
	return Decompress(data)
}

type Persister struct {
	dataDir string
}

func NewPersister(dataDir string) (*Persister, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir %s: %w", dataDir, err)
	}
	return &Persister{dataDir: dataDir}, nil
}

func (p *Persister) SaveBlock(block *Block) (string, error) {
	data, err := block.Compress()
	if err != nil {
		return "", fmt.Errorf("compress block: %w", err)
	}

	fileName := fmt.Sprintf("block_%d_%d.bin", block.MinTime, block.MaxTime)
	filePath := filepath.Join(p.dataDir, fileName)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("write block file %s: %w", filePath, err)
	}

	return filePath, nil
}

func (p *Persister) LoadBlock(filePath string) (*Block, error) {
	return ReadBlockFromFile(filePath)
}

func (p *Persister) DataDir() string {
	return p.dataDir
}
