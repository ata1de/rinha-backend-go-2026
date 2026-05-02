package knn

const (
	K    = 5
	DIMS = 16
)

// KNN5 retorna o número de vizinhos rotulados como fraude (0..5) entre os K=5
// mais próximos do query no dataset. O handler converte isso em fraud_score
// e em decisão approved/denied.
//
// Algoritmo:
//   - Sequencial (uma única goroutine). Em container de 0.45 CPU, paralelismo
//     intra-request só adiciona overhead — a concorrência inter-request já
//     satura a CPU disponível.

//   - Distância euclidiana ao quadrado em float32 puro (sem cast pra float64).

//   - Top-K mantido em array de tamanho fixo K com seleção O(K) do pior.

//   - Early-exit a cada bloco de 4 dimensões: se a soma parcial já excede
//     o pior do top-K atual, abandona o vetor sem ler o restante.
func KNN5(query [DIMS]float32, vectors []float32, labels []uint8, count int) uint8 {
	const inf = float32(1e30)

	var dists [K]float32
	var labs [K]uint8

	for i := 0; i < K; i++ {
		dists[i] = inf
	}

	maxDist := inf
	maxIdx := 0

	q0, q1, q2, q3 := query[0], query[1], query[2], query[3]
	q4, q5, q6, q7 := query[4], query[5], query[6], query[7]
	q8, q9, q10, q11 := query[8], query[9], query[10], query[11]
	q12, q13 := query[12], query[13]

	for i := 0; i < count; i++ {
		base := i * DIMS
		// Bloco 1 (dims 0-3)
		v := vectors[base : base+DIMS : base+DIMS] // BCE hint
		d0 := q0 - v[0]
		d1 := q1 - v[1]
		d2 := q2 - v[2]
		d3 := q3 - v[3]

		dist := d0*d0 + d1*d1 + d2*d2 + d3*d3

		if dist >= maxDist {
			continue
		}

		// Bloco 2 (dims 4-7)
		d0 = q4 - v[4]
		d1 = q5 - v[5]
		d2 = q6 - v[6]
		d3 = q7 - v[7]

		dist += d0*d0 + d1*d1 + d2*d2 + d3*d3

		if dist >= maxDist {
			continue
		}

		// Bloco 3 (dims 8-11)
		d0 = q8 - v[8]
		d1 = q9 - v[9]
		d2 = q10 - v[10]
		d3 = q11 - v[11]

		dist += d0*d0 + d1*d1 + d2*d2 + d3*d3

		if dist >= maxDist {
			continue
		}

		// Bloco 4 (dims 12-13). 14,15 são padding zero — não somamos.
		d0 = q12 - v[12]
		d1 = q13 - v[13]

		dist += d0*d0 + d1*d1

		if dist >= maxDist {
			continue
		}

		dists[maxIdx] = dist
		labs[maxIdx] = labels[i]

		// Recalcula o pior do top-K (5 comparações).
		maxDist = dists[0]
		maxIdx = 0

		if dists[1] > maxDist {
			maxDist = dists[1]
			maxIdx = 1
		}

		if dists[2] > maxDist {
			maxDist = dists[2]
			maxIdx = 2
		}

		if dists[3] > maxDist {
			maxDist = dists[3]
			maxIdx = 3
		}

		if dists[4] > maxDist {
			maxDist = dists[4]
			maxIdx = 4
		}
	}

	return labs[0] + labs[1] + labs[2] + labs[3] + labs[4]
}
