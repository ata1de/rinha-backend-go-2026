package vectorize

import (
	"encoding/json"
	"os"
)

type normalization struct {
	MaxAmount            float32 `json:"max_amount"`
	MaxInstallments      float32 `json:"max_installments"`
	AmountVsAvgRatio     float32 `json:"amount_vs_avg_ratio"`
	MaxMinutes           float32 `json:"max_minutes"`
	MaxKm                float32 `json:"max_km"`
	MaxTxCount24h        float32 `json:"max_tx_count_24h"`
	MaxMerchantAvgAmount float32 `json:"max_merchant_avg_amount"`
}

// Pré-computamos o inverso pra trocar divisão por multiplicação no hot path.
type normInv struct {
	InvMaxAmount, InvMaxInstallments, InvAmountVsAvg float32
	InvMaxMinutes, InvMaxKm, InvMaxTxCount24h        float32
	InvMaxMerchantAvgAmount                          float32
}

var (
	norm    normalization
	invNorm normInv

	// MCC é numérico de até 4 dígitos. Array de 10000 entradas
	// elimina hash lookup e cabe em ~40KB. -1 = MCC desconhecido (usa default 0.5).
	mccRiskArr [10000]float32
	mccDefault float32 = 0.5
)

func Init(normPath, mccPath string) error {
	nb, err := os.ReadFile(normPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(nb, &norm); err != nil {
		return err
	}
	invNorm = normInv{
		InvMaxAmount:            1.0 / norm.MaxAmount,
		InvMaxInstallments:      1.0 / norm.MaxInstallments,
		InvAmountVsAvg:          1.0 / norm.AmountVsAvgRatio,
		InvMaxMinutes:           1.0 / norm.MaxMinutes,
		InvMaxKm:                1.0 / norm.MaxKm,
		InvMaxTxCount24h:        1.0 / norm.MaxTxCount24h,
		InvMaxMerchantAvgAmount: 1.0 / norm.MaxMerchantAvgAmount,
	}

	mb, err := os.ReadFile(mccPath)
	if err != nil {
		return err
	}
	raw := map[string]float32{}
	if err := json.Unmarshal(mb, &raw); err != nil {
		return err
	}

	for i := range mccRiskArr {
		mccRiskArr[i] = mccDefault
	}
	for k, v := range raw {
		idx := parseMCC(k)
		if idx >= 0 && idx < len(mccRiskArr) {
			mccRiskArr[idx] = v
		}
	}
	return nil
}

// parseMCC converte uma string numérica (até 4 dígitos) em int.
// Retorna -1 se contiver caractere não numérico — caímos no default.
func parseMCC(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
		if n >= 10000 {
			return -1
		}
	}
	return n
}

func clamp(x float32) float32 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
