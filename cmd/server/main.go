package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"rinha2026/cmd/internal/dataset"
	"rinha2026/cmd/internal/vectorize"
)

var (
	db    *dataset.DB
	ready bool
)

// Respostas pré-fabricadas: fraud_score só assume 6 valores possíveis (0/5..5/5).
// Threshold de aprovação é score < 0.6 → contagem < 3 → approved=true.
// Pré-computar elimina marshaling de JSON e qualquer alocação no hot path.
var fraudResponses = [6][]byte{
	[]byte(`{"approved":true,"fraud_score":0.0}`),
	[]byte(`{"approved":true,"fraud_score":0.2}`),
	[]byte(`{"approved":true,"fraud_score":0.4}`),
	[]byte(`{"approved":false,"fraud_score":0.6}`),
	[]byte(`{"approved":false,"fraud_score":0.8}`),
	[]byte(`{"approved":false,"fraud_score":1.0}`),
}

// Pool de buffers pra ler o body sem alocar a cada request.
var bodyBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 2048)
		return &b
	},
}

// Pool de Payloads pra reaproveitar a struct de decode.
// Os slices de KnownMerchants são reaproveitados pelo encoding/json.
var payloadPool = sync.Pool{
	New: func() any {
		return &vectorize.Payload{}
	},
}

func main() {
	// Em container de 0.45 CPU, GOMAXPROCS=1 evita thrash de scheduler
	// e elimina a contenção de Ps competindo pela mesma fração de CPU.
	if v := os.Getenv("GOMAXPROCS"); v == "" {
		runtime.GOMAXPROCS(1)
	}

	// GC menos frequente — heap base ~6.4MB de vetores, GOGC=300 reduz
	// pauses sem estourar o limite de 160MB do container.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(300)
	}

	if err := vectorize.Init(
		"./resources/normalization.json",
		"./resources/mcc_risk.json",
	); err != nil {
		log.Fatal("vectorize.Init:", err)
	}

	var err error
	db, err = dataset.Load("./resources/references.json.gz")
	if err != nil {
		log.Fatal("dataset.Load:", err)
	}

	// Aquece o caminho quente: garante código JIT/inlinado e
	// que páginas estão residentes antes do primeiro tráfego real.
	warmup()

	ready = true
	log.Printf("Dataset carregado: %d vetores", db.Count)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", handleReady)
	mux.HandleFunc("POST /fraud-score", handleFraudScore)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	log.Printf("Servidor rodando na porta %s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func warmup() {
	p := vectorize.Payload{
		Transaction: vectorize.Transaction{Amount: 384.88, Installments: 3, RequestedAt: "2026-03-11T20:23:35Z"},
		Customer:    vectorize.Customer{AvgAmount: 769.76, TxCount24h: 3, KnownMerchants: []string{"MERC-009", "MERC-001"}},
		Merchant:    vectorize.Merchant{ID: "MERC-001", MCC: "5912", AvgAmount: 298.95},
		Terminal:    vectorize.Terminal{IsOnline: false, CardPresent: true, KmFromHome: 13.71},
		LastTx:      &vectorize.LastTx{Timestamp: "2026-03-11T14:58:35Z", KmFromCurrent: 18.86},
	}
	for i := 0; i < 200; i++ {
		v := vectorize.Vectorize(&p)
		_ = db.Tree.Search(v) // VP Tree já está pronta em db — só consulta
	}
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	if !ready {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleFraudScore(w http.ResponseWriter, r *http.Request) {
	bufPtr := bodyBufPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	defer func() {
		*bufPtr = buf[:0]
		bodyBufPool.Put(bufPtr)
	}()

	// Lê o body inteiro num buffer reutilizável; payload típico tem <2KB.
	for {
		if cap(buf)-len(buf) < 256 {
			nb := make([]byte, len(buf), cap(buf)*2+256)
			copy(nb, buf)
			buf = nb
		}
		n, err := r.Body.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}

	p := payloadPool.Get().(*vectorize.Payload)
	// Reset do que importa — outros campos serão sobrescritos pelo Unmarshal.
	p.LastTx = nil
	if err := json.Unmarshal(buf, p); err != nil {
		payloadPool.Put(p)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	vec := vectorize.Vectorize(p)
	count := db.Tree.Search(vec) // O(log N) via VP Tree em vez de O(N) scan linear
	payloadPool.Put(p)

	if count > 5 {
		count = 5
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(fraudResponses[count])
}
