package search

import "math"

const (
	K        = 5
	DIMS     = 16
	leafSize = 32
)

type vpNode struct {
	vpIdx     int32
	mu        float32
	inner     int32
	outer     int32
	leafStart int32
	leafLen   int32
}

type VPTree struct {
	nodes   []vpNode
	leafIdx []int32
	vectors []float32
	labels  []uint8
}

func Build(vectors []float32, labels []uint8, count int) *VPTree {
	t := &VPTree{
		nodes:   make([]vpNode, 0, count),
		leafIdx: make([]int32, 0, count),
		vectors: vectors,
		labels:  labels,
	}

	indices := make([]int32, count)

	for i := range indices {
		indices[i] = int32(i)
	}

	t.buildNode(indices)

	return t
}

func (t *VPTree) buildNode(indices []int32) int32 {
	nodeIdx := int32(len(t.nodes))
	t.nodes = append(t.nodes, vpNode{inner: -1, outer: -1})

	// Caso base: folha
	// Se o conjunto tem poucos pontos, salva os índices diretamente na folha. Na busca, esses pontos serão comparados via brute-force (rápido para N pequeno).
	if len(indices) <= leafSize {
		start := int32(len(t.leafIdx))

		t.leafIdx = append(t.leafIdx, indices...)

		t.nodes[nodeIdx].leafStart = start
		t.nodes[nodeIdx].leafLen = int32(len(indices))
		t.nodes[nodeIdx].vpIdx = indices[0] // necessário ter um índice válido

		return nodeIdx
	}

	// Escolha do Vantage Point (VP)
	// O VP é o ponto em torno do qual particionamos o espaço.

	vp := indices[0]
	t.nodes[nodeIdx].vpIdx = vp
	rest := indices[1:]

	dists := make([]float32, len(rest))

	for i, idx := range rest {
		dists[i] = distBetween(t.vectors, int(vp), int(idx))
	}

	// Encontra a mediana (mu) via quickselect
	// mu é o raio que divide inner (mais próximos do VP) de outer (mais distantes).
	mid := len(rest) / 2
	nthElement(dists, rest, mid)
	mu := dists[mid]

	t.nodes[nodeIdx].mu = mu

	// Particiona em inner (dist ≤ mu) e outer (dist > mu)
	innerIdx := rest[:0]
	outerIdx := make([]int32, 0, len(rest)-mid)

	for i, d := range dists {
		if d <= mu {
			innerIdx = append(innerIdx, rest[i])
		} else {
			outerIdx = append(outerIdx, rest[i])
		}
	}

	if len(innerIdx) > 0 {
		t.nodes[nodeIdx].inner = t.buildNode(innerIdx)
	}

	if len(outerIdx) > 0 {
		t.nodes[nodeIdx].outer = t.buildNode(outerIdx)
	}

	return nodeIdx
}

// Search — busca os K vizinhos mais próximos em O(log N) médio

// Search retorna o número de vizinhos rotulados como fraude (0..K) entre os
// K=5 mais próximos do query na VP Tree. Mesma semântica do antigo KNN5.
func (t *VPTree) Search(query [DIMS]float32) uint8 {
	const unfilledDistance = float32(1e30)

	var neighborDistances [K]float32
	var neighborLabels [K]uint8

	for i := range neighborDistances {
		neighborDistances[i] = unfilledDistance
	}

	worstNeighborDistance := unfilledDistance
	furthestNeighborSlot := 0

	t.searchNode(0, query, &neighborDistances, &neighborLabels, &worstNeighborDistance, &furthestNeighborSlot)

	// Soma os labels dos K vizinhos (0 ou 1 cada) → contagem de fraudes (0..5).
	return neighborLabels[0] + neighborLabels[1] + neighborLabels[2] + neighborLabels[3] + neighborLabels[4]
}

// searchNode desce recursivamente na árvore, podando ramos que a desigualdade
// triangular garante não conter vizinhos melhores que worstNeighborDistance.
func (t *VPTree) searchNode(
	nodeIdx int32,
	query [DIMS]float32,
	neighborDistances *[K]float32,
	neighborLabels *[K]uint8,
	worstNeighborDistance *float32,
	furthestNeighborSlot *int,
) {
	node := &t.nodes[nodeIdx]

	// Caso: folha
	// Faz brute-force nos pontos da folha usando distância ao quadrado
	// (sem sqrt), comparando contra worstNeighborDistance² — evita sqrt desnecessário.
	if node.leafLen > 0 {
		worstDistSq := (*worstNeighborDistance) * (*worstNeighborDistance)
		for _, idx := range t.leafIdx[node.leafStart : node.leafStart+node.leafLen] {
			dsq := squaredDist(query, t.vectors, int(idx))

			if dsq < worstDistSq {
				// Encontrou vizinho mais próximo: substitui o pior slot do top-K.

				dist := float32(math.Sqrt(float64(dsq)))
				(*neighborDistances)[*furthestNeighborSlot] = dist
				(*neighborLabels)[*furthestNeighborSlot] = t.labels[idx]
				updateWorst(neighborDistances, worstNeighborDistance, furthestNeighborSlot)
				worstDistSq = (*worstNeighborDistance) * (*worstNeighborDistance)
			}
		}

		return
	}

	// Nó interno: calcula distância real do query ao VP
	distToVP := distFromQuery(query, t.vectors, int(node.vpIdx))

	// Tenta inserir o próprio VP no top-K.
	if distToVP < *worstNeighborDistance {
		(*neighborDistances)[*furthestNeighborSlot] = distToVP
		(*neighborLabels)[*furthestNeighborSlot] = t.labels[node.vpIdx]

		updateWorst(neighborDistances, worstNeighborDistance, furthestNeighborSlot)
	}

	// Poda por desigualdade triangular
	//
	// Seja p um ponto qualquer e VP o vantage point. Pela desigualdade triangular:
	//   dist(query, p) ≥ |dist(query, VP) - dist(VP, p)|

	mu := node.mu

	if distToVP <= mu {
		// Query está mais perto do VP que mu → inner é o ramo natural.
		if node.inner != -1 && distToVP-*worstNeighborDistance <= mu {
			t.searchNode(node.inner, query, neighborDistances, neighborLabels, worstNeighborDistance, furthestNeighborSlot)
		}

		if node.outer != -1 && distToVP+*worstNeighborDistance >= mu {
			t.searchNode(node.outer, query, neighborDistances, neighborLabels, worstNeighborDistance, furthestNeighborSlot)
		}
	} else {
		// Query está além de mu → outer é o ramo natural.
		if node.outer != -1 && distToVP+*worstNeighborDistance >= mu {
			t.searchNode(node.outer, query, neighborDistances, neighborLabels, worstNeighborDistance, furthestNeighborSlot)
		}

		if node.inner != -1 && distToVP-*worstNeighborDistance <= mu {
			t.searchNode(node.inner, query, neighborDistances, neighborLabels, worstNeighborDistance, furthestNeighborSlot)
		}
	}
}

// updateWorst recalcula qual slot do top-K tem a maior distância (o pior).
// Chamado após cada inserção no top-K. Apenas K=5 comparações fixas.
func updateWorst(neighborDistances *[K]float32, worstNeighborDistance *float32, furthestNeighborSlot *int) {
	*worstNeighborDistance = neighborDistances[0]
	*furthestNeighborSlot = 0

	for slot := 1; slot < K; slot++ {
		if neighborDistances[slot] > *worstNeighborDistance {
			*worstNeighborDistance = neighborDistances[slot]
			*furthestNeighborSlot = slot
		}
	}
}

// distBetween retorna a distância Euclidiana real entre os pontos a e b
func distBetween(vecs []float32, a, b int) float32 {
	offsetA, offsetB := a*DIMS, b*DIMS
	var sumOfSquares float64

	for dim := 0; dim < DIMS; dim++ {
		diff := float64(vecs[offsetA+dim]) - float64(vecs[offsetB+dim])

		sumOfSquares += diff * diff
	}

	return float32(math.Sqrt(sumOfSquares))
}

// distFromQuery retorna a distância Euclidiana real entre o query (array fixo) e o ponto idx no slice plano de vetores.
func distFromQuery(query [DIMS]float32, vecs []float32, idx int) float32 {
	off := idx * DIMS
	var sum float64
	for i := 0; i < DIMS; i++ {
		d := float64(query[i]) - float64(vecs[off+i])
		sum += d * d
	}
	return float32(math.Sqrt(sum))
}

// squaredDist retorna a distância ao quadrado entre o query e o ponto idx.
func squaredDist(query [DIMS]float32, vecs []float32, idx int) float32 {
	off := idx * DIMS
	var sum float32
	for i := 0; i < DIMS; i++ {
		d := query[i] - vecs[off+i]
		sum += d * d
	}
	return sum
}

// nthElement — seleciona a mediana em O(N) esperado (quickselect)

// nthElement reorganiza `dists` e `indices` de modo que dists[k] seja o valor
// que estaria na posição k numa ordenação completa, com todos os elementos à
// esquerda ≤ dists[k] e à direita ≥ dists[k].

// quickselect faz O(N) em média — sem ordenar o que não precisamos.
func nthElement(dists []float32, indices []int32, k int) {
	subStart, subEnd := 0, len(dists)-1

	for subStart < subEnd {
		pivotDist := dists[(subStart+subEnd)/2]

		left, right := subStart, subEnd

		for left <= right {
			for dists[left] < pivotDist {
				left++
			}

			for dists[right] > pivotDist {
				right--
			}

			if left <= right {
				dists[left], dists[right] = dists[right], dists[left]

				indices[left], indices[right] = indices[right], indices[left]

				left++
				right--
			}
		}

		if right < k {
			subStart = left
		}

		if left > k {
			subEnd = right
		}

		if right < k && left > k {
			break
		}
	}
}
