package main

import (
	"fmt"
	"rinha2026/cmd/internal/dataset"
	"rinha2026/cmd/internal/knn"
	"rinha2026/cmd/internal/vectorize"
)

func main() {
	vectorize.Init("./resources/normalization.json", "./resources/mcc_risk.json")
	db, _ := dataset.Load("./resources/references.json.gz")

	cases := []struct {
		name         string
		payload      vectorize.Payload
		wantApproved bool
	}{
		{
			name:         "legítima",
			wantApproved: true,
			payload: vectorize.Payload{
				Transaction: vectorize.Transaction{Amount: 41.12, Installments: 2, RequestedAt: "2026-03-11T18:45:53Z"},
				Customer:    vectorize.Customer{AvgAmount: 82.24, TxCount24h: 3, KnownMerchants: []string{"MERC-003", "MERC-016"}},
				Merchant:    vectorize.Merchant{ID: "MERC-016", MCC: "5411", AvgAmount: 60.25},
				Terminal:    vectorize.Terminal{IsOnline: false, CardPresent: true, KmFromHome: 29.23},
			},
		},
		{
			name:         "fraudulenta",
			wantApproved: false,
			payload: vectorize.Payload{
				Transaction: vectorize.Transaction{Amount: 9505.97, Installments: 10, RequestedAt: "2026-03-14T05:15:12Z"},
				Customer:    vectorize.Customer{AvgAmount: 81.28, TxCount24h: 20, KnownMerchants: []string{"MERC-008", "MERC-007", "MERC-005"}},
				Merchant:    vectorize.Merchant{ID: "MERC-068", MCC: "7802", AvgAmount: 54.86},
				Terminal:    vectorize.Terminal{IsOnline: false, CardPresent: true, KmFromHome: 952.27},
			},
		},
	}

	for _, c := range cases {
		vec := vectorize.Vectorize(&c.payload)
		count := knn.KNN5(vec, db.Vectors, db.Labels, db.Count)
		score := float32(count) / 5.0
		approved := count < 3
		status := "✓"
		if approved != c.wantApproved {
			status = "✗ ERRADO"
		}
		fmt.Printf("%s %s: fraud_score=%.1f approved=%v\n", status, c.name, score, approved)
	}
}
