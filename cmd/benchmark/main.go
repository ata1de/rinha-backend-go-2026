// cmd/benchmark/main.go
package main

import (
	"fmt"
	"rinha2026/cmd/internal/dataset"
	"rinha2026/cmd/internal/vectorize"
	"time"
)

func main() {
	vectorize.Init("./resources/normalization.json", "./resources/mcc_risk.json")
	db, _ := dataset.Load("./resources/references.json.gz")

	p := vectorize.Payload{
		Transaction: vectorize.Transaction{Amount: 384.88, Installments: 3, RequestedAt: "2026-03-11T20:23:35Z"},
		Customer:    vectorize.Customer{AvgAmount: 769.76, TxCount24h: 3, KnownMerchants: []string{"MERC-009", "MERC-001"}},
		Merchant:    vectorize.Merchant{ID: "MERC-001", MCC: "5912", AvgAmount: 298.95},
		Terminal:    vectorize.Terminal{IsOnline: false, CardPresent: true, KmFromHome: 13.71},
		LastTx:      &vectorize.LastTx{Timestamp: "2026-03-11T14:58:35Z", KmFromCurrent: 18.86},
	}
	vec := vectorize.Vectorize(&p)

	// Aquecimento do JIT
	var sink uint8
	for i := 0; i < 500; i++ {
		sink += db.Tree.Search(vec)
	}

	// Benchmark
	N := 2000
	start := time.Now()
	for i := 0; i < N; i++ {
		sink += db.Tree.Search(vec)
	}
	_ = sink
	elapsed := time.Since(start)
	avg := elapsed / time.Duration(N)
	fmt.Printf("Média por query: %v\n", avg)
	fmt.Printf("p99 estimado:    %v\n", avg*14/10) // p99 ≈ 1.4× média
}
