package consensus

import (
	"sync"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// DAG represents a directed acyclic graph of vertices
type DAG struct {
	// Genesis vertex
	genesis *model.Vertex

	// Vertices by round
	vertices map[model.Round]map[model.VertexHash]*model.Vertex

	// Edges between vertices
	edges map[model.VertexHash]map[model.VertexHash]struct{}

	// Strong edges
	strongEdges map[model.VertexHash]map[model.VertexHash]struct{}

	// Weak edges
	weakEdges map[model.VertexHash]map[model.VertexHash]struct{}

	mu sync.RWMutex
}

// NewDAG creates a new DAG with a genesis vertex
func NewDAG(genesis *model.Vertex) *DAG {
	dag := &DAG{
		genesis:     genesis,
		vertices:    make(map[model.Round]map[model.VertexHash]*model.Vertex),
		edges:       make(map[model.VertexHash]map[model.VertexHash]struct{}),
		strongEdges: make(map[model.VertexHash]map[model.VertexHash]struct{}),
		weakEdges:   make(map[model.VertexHash]map[model.VertexHash]struct{}),
	}

	// Add genesis vertex
	dag.vertices[0] = make(map[model.VertexHash]*model.Vertex)
	dag.vertices[0][genesis.Hash()] = genesis

	return dag
}

// InsertVertex adds a vertex to the DAG
func (d *DAG) InsertVertex(vertex *model.Vertex) {
	// Initialize round map if needed
	if _, ok := d.vertices[vertex.Round()]; !ok {
		d.vertices[vertex.Round()] = make(map[model.VertexHash]*model.Vertex)
	}

	// Add vertex
	d.vertices[vertex.Round()][vertex.Hash()] = vertex

	// Add strong edges
	d.strongEdges[vertex.Hash()] = make(map[model.VertexHash]struct{})
	for parentHash := range vertex.Parents() {
		d.strongEdges[vertex.Hash()][parentHash] = struct{}{}
	}

	// Add weak edges for rounds > 2
	if vertex.Round() > 2 {
		d.addWeakEdges(vertex)
	}
}

// addWeakEdges adds weak edges for a vertex
func (d *DAG) addWeakEdges(vertex *model.Vertex) {
	d.weakEdges[vertex.Hash()] = make(map[model.VertexHash]struct{})

	// Add weak edges to vertices in previous rounds
	for r := model.Round(1); r < vertex.Round()-2; r++ {
		if vertices, ok := d.vertices[r]; ok {
			for _, v := range vertices {
				// Skip if there's a direct strong edge
				if _, hasStrongEdge := d.strongEdges[vertex.Hash()][v.Hash()]; !hasStrongEdge {
					d.weakEdges[vertex.Hash()][v.Hash()] = struct{}{}
				}
			}
		}
	}
}

// ContainsVertices checks if all vertices exist in the DAG
func (d *DAG) ContainsVertices(hashes []model.VertexHash) bool {
	for _, hash := range hashes {
		found := false
		for _, vertices := range d.vertices {
			if _, ok := vertices[hash]; ok {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// GetVertexCountInRound returns the number of vertices in a round
func (d *DAG) GetVertexCountInRound(round model.Round) int {
	if vertices, ok := d.vertices[round]; ok {
		return len(vertices)
	}
	return 0
}

// GetVerticesInRound returns all vertices in a round
func (d *DAG) GetVerticesInRound(round model.Round) []*model.Vertex {
	if vertices, ok := d.vertices[round]; ok {
		result := make([]*model.Vertex, 0, len(vertices))
		for _, v := range vertices {
			result = append(result, v)
		}
		return result
	}
	return nil
}

// GetStrongParents returns strong parents for a round
func (d *DAG) GetStrongParents(round model.Round) []model.VertexHash {
	if round <= 0 {
		return nil
	}

	if vertices, ok := d.vertices[round-1]; ok {
		parents := make([]model.VertexHash, 0, len(vertices))
		for _, v := range vertices {
			parents = append(parents, v.Hash())
		}
		return parents
	}
	return nil
}

// IsLinked checks if there is a path from v1 to v2
func (d *DAG) IsLinked(v1, v2 *model.Vertex) bool {
	// Check strong edges
	if edges, ok := d.strongEdges[v1.Hash()]; ok {
		if _, ok := edges[v2.Hash()]; ok {
			return true
		}
	}

	// Check weak edges
	if edges, ok := d.weakEdges[v1.Hash()]; ok {
		if _, ok := edges[v2.Hash()]; ok {
			return true
		}
	}

	// Add weak edges for vertices with round gaps > 2
	if v1.Round() > v2.Round() && v1.Round()-v2.Round() > 2 {
		if _, ok := d.vertices[v2.Round()][v2.Hash()]; ok {
			// Add weak edge
			if d.weakEdges[v1.Hash()] == nil {
				d.weakEdges[v1.Hash()] = make(map[model.VertexHash]struct{})
			}
			d.weakEdges[v1.Hash()][v2.Hash()] = struct{}{}
			return true
		}
	}

	return false
}

// IsStronglyLinked checks if there is a strong path from v1 to v2
func (d *DAG) IsStronglyLinked(v1, v2 *model.Vertex) bool {
	if edges, ok := d.strongEdges[v1.Hash()]; ok {
		if _, ok := edges[v2.Hash()]; ok {
			return true
		}
	}
	return false
}

// IsLinkedWithOthersInRound checks if a vertex is linked with others in a round
func (d *DAG) IsLinkedWithOthersInRound(vertex *model.Vertex, round model.Round) bool {
	if vertices, ok := d.vertices[round]; ok {
		for _, v := range vertices {
			if v.Hash() != vertex.Hash() && d.IsLinked(vertex, v) {
				return true
			}
		}
	}
	return false
}

// GetWaveVertexLeader returns the leader vertex for a wave
func (d *DAG) GetWaveVertexLeader(wave model.Wave) *model.Vertex {
	d.mu.RLock()
	defer d.mu.RUnlock()

	round := model.Round(wave * MaxWave)
	if round == 0 {
		return d.genesis
	}

	vertices := d.GetVerticesInRound(round)
	if len(vertices) == 0 {
		return nil
	}

	// Return the first vertex in the round as the leader
	return vertices[0]
}

// GetVertices returns all vertices in the DAG
func (d *DAG) GetVertices() map[model.Round]map[model.VertexHash]*model.Vertex {
	return d.vertices
}
