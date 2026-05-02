package vectorize

type Payload struct {
	ID          string      `json:"id"`
	Transaction Transaction `json:"transaction"`
	Customer    Customer    `json:"customer"`
	Merchant    Merchant    `json:"merchant"`
	Terminal    Terminal    `json:"terminal"`
	LastTx      *LastTx     `json:"last_transaction"`
}

type Transaction struct {
	Amount       float32 `json:"amount"`
	Installments float32 `json:"installments"`
	RequestedAt  string  `json:"requested_at"`
}

type Customer struct {
	AvgAmount      float32  `json:"avg_amount"`
	TxCount24h     float32  `json:"tx_count_24h"`
	KnownMerchants []string `json:"known_merchants"`
}

type Merchant struct {
	ID        string  `json:"id"`
	MCC       string  `json:"mcc"`
	AvgAmount float32 `json:"avg_amount"`
}

type Terminal struct {
	IsOnline    bool    `json:"is_online"`
	CardPresent bool    `json:"card_present"`
	KmFromHome  float32 `json:"km_from_home"`
}

type LastTx struct {
	Timestamp     string  `json:"timestamp"`
	KmFromCurrent float32 `json:"km_from_current"`
}

func Vectorize(p *Payload) [16]float32 {
	var vec [16]float32

	cur, okCur := parseFastTS(p.Transaction.RequestedAt)

	vec[0] = clamp(p.Transaction.Amount * invNorm.InvMaxAmount)

	vec[1] = clamp(p.Transaction.Installments * invNorm.InvMaxInstallments)

	if p.Customer.AvgAmount > 0 {
		vec[2] = clamp((p.Transaction.Amount / p.Customer.AvgAmount) * invNorm.InvAmountVsAvg)
	}

	if okCur {
		vec[3] = float32(cur.hour) * (1.0 / 23.0)

		vec[4] = float32(cur.weekdayMonZero) * (1.0 / 6.0)
	}

	if p.LastTx == nil || !okCur {
		vec[5] = -1
		vec[6] = -1
	} else {
		last, okLast := parseFastTS(p.LastTx.Timestamp)
		if !okLast {
			vec[5] = -1
			vec[6] = -1
		} else {
			minutes := float32(cur.epochSec-last.epochSec) * (1.0 / 60.0)
			vec[5] = clamp(minutes * invNorm.InvMaxMinutes)
			vec[6] = clamp(p.LastTx.KmFromCurrent * invNorm.InvMaxKm)
		}
	}

	vec[7] = clamp(p.Terminal.KmFromHome * invNorm.InvMaxKm)

	vec[8] = clamp(p.Customer.TxCount24h * invNorm.InvMaxTxCount24h)

	if p.Terminal.IsOnline {
		vec[9] = 1
	}

	if p.Terminal.CardPresent {
		vec[10] = 1
	}

	vec[11] = 1
	mid := p.Merchant.ID
	for i := range p.Customer.KnownMerchants {
		if p.Customer.KnownMerchants[i] == mid {
			vec[11] = 0
			break
		}
	}

	if idx := parseMCC(p.Merchant.MCC); idx >= 0 {
		vec[12] = mccRiskArr[idx]
	} else {
		vec[12] = mccDefault
	}

	vec[13] = clamp(p.Merchant.AvgAmount * invNorm.InvMaxMerchantAvgAmount)

	return vec
}
