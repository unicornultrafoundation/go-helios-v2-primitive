package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// ===== TEST HELPERS =====

// createTestVertex creates a vertex with the specified round and parents
func createTestVertex(t *testing.T, owner model.NodePublicKey, round model.Round, block *model.Block, parents map[model.VertexHash]model.Round) *model.Vertex {
	return model.NewVertex(owner, round, block, parents)
}

// ===== DAG TESTS =====

// TestDAGCreation tests creating a new DAG
func TestDAGCreation(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Verify DAG is created correctly
	assert.NotNil(t, dag, "DAG should not be nil")
	assert.Equal(t, genesis, dag.genesis, "DAG genesis should match")
	assert.Equal(t, 1, len(dag.vertices), "DAG should have one round")
	assert.Equal(t, 1, len(dag.vertices[0]), "DAG should have one vertex in round 0")
	assert.Equal(t, genesis, dag.vertices[0][genesis.Hash()], "DAG should have the genesis vertex")
}

// TestInsertVertex tests inserting vertices into the DAG
func TestInsertVertex(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Verify vertex was inserted correctly
	assert.Equal(t, 2, len(dag.vertices), "DAG should have two rounds")
	assert.Equal(t, 1, len(dag.vertices[1]), "DAG should have one vertex in round 1")
	assert.Equal(t, vertex1, dag.vertices[1][vertex1.Hash()], "DAG should have the vertex in round 1")

	// Check strong edges
	assert.Equal(t, 1, len(dag.strongEdges[vertex1.Hash()]), "Vertex should have one strong edge")
	_, exists := dag.strongEdges[vertex1.Hash()][genesis.Hash()]
	assert.True(t, exists, "Strong edge to genesis should exist")

	// Create a vertex in round 2
	block2 := model.NewBlock(2, []model.Transaction{CreateTestTransaction(64)})
	parents2 := map[model.VertexHash]model.Round{
		vertex1.Hash(): 1,
	}
	vertex2 := createTestVertex(t, nodeKey, 2, block2, parents2)

	// Insert vertex into DAG
	dag.InsertVertex(vertex2)

	// Verify vertex was inserted correctly
	assert.Equal(t, 3, len(dag.vertices), "DAG should have three rounds")
	assert.Equal(t, 1, len(dag.vertices[2]), "DAG should have one vertex in round 2")
	assert.Equal(t, vertex2, dag.vertices[2][vertex2.Hash()], "DAG should have the vertex in round 2")

	// Check strong edges
	assert.Equal(t, 1, len(dag.strongEdges[vertex2.Hash()]), "Vertex should have one strong edge")
	_, exists = dag.strongEdges[vertex2.Hash()][vertex1.Hash()]
	assert.True(t, exists, "Strong edge to vertex1 should exist")
}

// TestInsertVertexWithWeakEdges tests inserting vertices with weak edges
func TestInsertVertexWithWeakEdges(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Create vertices for rounds 1-4
	var vertices []*model.Vertex
	var lastVertex *model.Vertex = genesis

	for round := model.Round(1); round <= 4; round++ {
		block := model.NewBlock(uint64(round), []model.Transaction{CreateTestTransaction(64)})
		parents := map[model.VertexHash]model.Round{
			lastVertex.Hash(): round - 1,
		}
		vertex := createTestVertex(t, nodeKey, round, block, parents)
		vertices = append(vertices, vertex)
		dag.InsertVertex(vertex)
		lastVertex = vertex
	}

	// Check weak edges for vertex in round 4
	// It should have weak edges to rounds 1
	vertex4 := vertices[3] // 0-indexed, so 4th vertex is at index 3
	assert.Equal(t, 1, len(dag.weakEdges[vertex4.Hash()]), "Vertex in round 4 should have weak edges")
	_, exists := dag.weakEdges[vertex4.Hash()][vertices[0].Hash()]
	assert.True(t, exists, "Weak edge to vertex in round 1 should exist")
}

// TestContainsVertices tests checking if vertices exist in the DAG
func TestContainsVertices(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Check contains vertices
	hashes := []model.VertexHash{genesis.Hash(), vertex1.Hash()}
	assert.True(t, dag.ContainsVertices(hashes), "DAG should contain both vertices")

	// Create a vertex that isn't in the DAG
	block2 := model.NewBlock(2, []model.Transaction{CreateTestTransaction(64)})
	parents2 := map[model.VertexHash]model.Round{
		vertex1.Hash(): 1,
	}
	vertex2 := createTestVertex(t, nodeKey, 2, block2, parents2)

	// Check contains vertices with vertex not in DAG
	hashes = append(hashes, vertex2.Hash())
	assert.False(t, dag.ContainsVertices(hashes), "DAG should not contain all vertices")
}

// TestGetVertexCountInRound tests getting vertex count in a round
func TestGetVertexCountInRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Check vertex count in round 0
	assert.Equal(t, 1, dag.GetVertexCountInRound(0), "Round 0 should have 1 vertex")
	assert.Equal(t, 0, dag.GetVertexCountInRound(1), "Round 1 should have 0 vertices")

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Check vertex count in round 1
	assert.Equal(t, 1, dag.GetVertexCountInRound(1), "Round 1 should have 1 vertex")

	// Add another vertex to round 1
	nodeKey2 := GenerateMockPublicKey(2)
	block2 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(32)})
	vertex2 := createTestVertex(t, nodeKey2, 1, block2, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex2)

	// Check vertex count in round 1
	assert.Equal(t, 2, dag.GetVertexCountInRound(1), "Round 1 should have 2 vertices")
}

// TestGetVerticesInRound tests getting vertices in a round
func TestGetVerticesInRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Check vertices in round 0
	vertices := dag.GetVerticesInRound(0)
	require.Equal(t, 1, len(vertices), "Round 0 should have 1 vertex")
	assert.Equal(t, genesis, vertices[0], "Vertex in round 0 should be genesis")

	// Check vertices in round 1
	assert.Nil(t, dag.GetVerticesInRound(1), "Round 1 should have no vertices")

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Check vertices in round 1
	vertices = dag.GetVerticesInRound(1)
	require.Equal(t, 1, len(vertices), "Round 1 should have 1 vertex")
	assert.Equal(t, vertex1, vertices[0], "Vertex in round 1 should match")
}

// TestGetStrongParents tests getting strong parents for a round
func TestGetStrongParents(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Check strong parents for round 0
	parents := dag.GetStrongParents(0)
	assert.Nil(t, parents, "Round 0 should have no strong parents")

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parentMap := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parentMap)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Check strong parents for round 2
	parents = dag.GetStrongParents(2)
	require.Equal(t, 1, len(parents), "Round 2 should have 1 strong parent")
	assert.Equal(t, vertex1.Hash(), parents[0], "Strong parent should be vertex1")
}

// TestIsLinked tests if vertices are linked
func TestIsLinked(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Check if vertex1 is linked to genesis (it has a strong edge)
	assert.True(t, dag.IsLinked(vertex1, genesis), "Vertex1 should be linked to genesis")

	// Create a vertex in round 2
	block2 := model.NewBlock(2, []model.Transaction{CreateTestTransaction(32)})
	parents2 := map[model.VertexHash]model.Round{
		vertex1.Hash(): 1,
	}
	vertex2 := createTestVertex(t, nodeKey, 2, block2, parents2)

	// Insert vertex into DAG
	dag.InsertVertex(vertex2)

	// Check if vertex2 is linked to vertex1 (strong edge)
	assert.True(t, dag.IsLinked(vertex2, vertex1), "Vertex2 should be linked to vertex1")

	// Check if vertex2 is linked to genesis (via weak edge that gets added for rounds > 2)
	// This connection is through vertex1
	assert.False(t, dag.IsLinked(vertex2, genesis), "Vertex2 should not be directly linked to genesis yet")

	// Create a vertex in round 3 which should have weak edges to genesis
	block3 := model.NewBlock(3, []model.Transaction{CreateTestTransaction(32)})
	parents3 := map[model.VertexHash]model.Round{
		vertex2.Hash(): 2,
	}
	vertex3 := createTestVertex(t, nodeKey, 3, block3, parents3)

	// Insert vertex into DAG
	dag.InsertVertex(vertex3)

	// Verify weak edges to genesis (because of round gap > 2)
	assert.True(t, dag.IsLinked(vertex3, genesis), "Vertex3 should be linked to genesis with weak edge")
}

// TestIsStronglyLinked tests if vertices are strongly linked
func TestIsStronglyLinked(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// Check if vertex1 is strongly linked to genesis
	assert.True(t, dag.IsStronglyLinked(vertex1, genesis), "Vertex1 should be strongly linked to genesis")

	// Create a vertex in round 2
	block2 := model.NewBlock(2, []model.Transaction{CreateTestTransaction(32)})
	parents2 := map[model.VertexHash]model.Round{
		vertex1.Hash(): 1,
	}
	vertex2 := createTestVertex(t, nodeKey, 2, block2, parents2)

	// Insert vertex into DAG
	dag.InsertVertex(vertex2)

	// Check if vertex2 is strongly linked to vertex1
	assert.True(t, dag.IsStronglyLinked(vertex2, vertex1), "Vertex2 should be strongly linked to vertex1")

	// Check if vertex2 is strongly linked to genesis (it shouldn't be)
	assert.False(t, dag.IsStronglyLinked(vertex2, genesis), "Vertex2 should not be strongly linked to genesis")
}

// TestIsLinkedWithOthersInRound tests if a vertex is linked with others in a round
func TestIsLinkedWithOthersInRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex1)

	// With only one vertex in round 1, it's not linked with others in the round
	assert.False(t, dag.IsLinkedWithOthersInRound(vertex1, 1), "Vertex1 should not be linked with others in round 1")

	// Create another vertex in round 1
	nodeKey2 := GenerateMockPublicKey(2)
	block2 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(32)})
	vertex2 := createTestVertex(t, nodeKey2, 1, block2, parents)

	// Insert vertex into DAG
	dag.InsertVertex(vertex2)

	// With only strong parents, vertices in the same round aren't linked
	assert.False(t, dag.IsLinkedWithOthersInRound(vertex1, 1), "Vertices in same round aren't linked by default")

	// Create a vertex in round 2 with both vertices from round 1 as parents
	block3 := model.NewBlock(2, []model.Transaction{CreateTestTransaction(32)})
	parents3 := map[model.VertexHash]model.Round{
		vertex1.Hash(): 1,
		vertex2.Hash(): 1,
	}
	vertex3 := createTestVertex(t, nodeKey, 2, block3, parents3)

	// Insert vertex into DAG
	dag.InsertVertex(vertex3)

	// Now vertex3 should be linked with all vertices in round 1
	assert.True(t, dag.IsStronglyLinked(vertex3, vertex1), "Vertex3 should be strongly linked to vertex1")
	assert.True(t, dag.IsStronglyLinked(vertex3, vertex2), "Vertex3 should be strongly linked to vertex2")
}

// TestGetWaveVertexLeader tests getting the leader vertex for a wave
func TestGetWaveVertexLeader(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Test getting leader for wave 0 (genesis)
	leader := dag.GetWaveVertexLeader(0)
	assert.Equal(t, genesis, leader, "Wave 0 leader should be genesis vertex")

	// Create vertices for rounds corresponding to waves
	var lastVertex *model.Vertex = genesis
	for wave := model.Wave(1); wave <= 2; wave++ {
		round := model.Round(wave * MaxWave)
		block := model.NewBlock(uint64(round), []model.Transaction{CreateTestTransaction(64)})
		parents := map[model.VertexHash]model.Round{
			lastVertex.Hash(): lastVertex.Round(),
		}
		vertex := createTestVertex(t, nodeKey, round, block, parents)
		dag.InsertVertex(vertex)
		lastVertex = vertex

		// Check wave vertex leader
		leader := dag.GetWaveVertexLeader(wave)
		assert.NotNil(t, leader, "Leader for wave should not be nil")
		assert.Equal(t, vertex, leader, "Leader should be the vertex we just created")
		assert.Equal(t, round, leader.Round(), "Leader round should match wave round")
	}

	// Test getting leader for non-existent wave
	leader = dag.GetWaveVertexLeader(3)
	assert.Nil(t, leader, "Leader for non-existent wave should be nil")
}

// TestGetVertices tests getting all vertices in the DAG
func TestGetVertices(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := createTestVertex(t, nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create a DAG
	dag := NewDAG(genesis)

	// Get vertices from DAG
	vertices := dag.GetVertices()
	assert.Equal(t, 1, len(vertices), "DAG should have one round")
	assert.Equal(t, 1, len(vertices[0]), "DAG should have one vertex in round 0")
	assert.Equal(t, genesis, vertices[0][genesis.Hash()], "Vertex in round 0 should be genesis")

	// Add a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := createTestVertex(t, nodeKey, 1, block1, parents)
	dag.InsertVertex(vertex1)

	// Get vertices again
	vertices = dag.GetVertices()
	assert.Equal(t, 2, len(vertices), "DAG should have two rounds")
	assert.Equal(t, 1, len(vertices[1]), "DAG should have one vertex in round 1")
	assert.Equal(t, vertex1, vertices[1][vertex1.Hash()], "Vertex in round 1 should match")
}
