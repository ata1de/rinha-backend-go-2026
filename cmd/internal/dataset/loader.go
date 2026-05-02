package dataset

import (
	"compress/gzip"
	"encoding/json"
	"os"
)

type DB struct {
	Vectors []float32 // flat: count * 16 floats
	Labels  []uint8   // 0=legit, 1=fraud
	Count   int
}

type refEntry struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

func Load(path string) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	var entries []refEntry
	if err := json.NewDecoder(gz).Decode(&entries); err != nil {
		return nil, err
	}

	count := len(entries)
	vectors := make([]float32, count*16)
	labels := make([]uint8, count)

	for i, e := range entries {
		base := i * 16
		for d, v := range e.Vector {
			vectors[base+d] = v
		}
		// posições 14 e 15 ficam 0.0 — zero-value do Go
		if e.Label == "fraud" {
			labels[i] = 1
		}
	}

	db := &DB{Vectors: vectors, Labels: labels, Count: count}
	prefault(db)
	return db, nil
}

// prefault toca cada página dos slices pra evitar minor page faults
// na primeira request (que causariam spike no p99 inicial).
func prefault(db *DB) {
	const pageF32 = 4096 / 4 // 1024 float32 por página de 4KB
	var sink float32
	for i := 0; i < len(db.Vectors); i += pageF32 {
		sink += db.Vectors[i]
	}
	const pageU8 = 4096
	var sinkU uint8
	for i := 0; i < len(db.Labels); i += pageU8 {
		sinkU += db.Labels[i]
	}
	// uso noop pra impedir DCE
	if sink == 1234567 && sinkU == 255 {
		println("unreachable")
	}
}
