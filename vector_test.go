package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorString(t *testing.T) {
	v := psql.Vector{1.0, 2.5, 3.75}
	assert.Equal(t, "[1,2.5,3.75]", v.String())

	v2 := psql.Vector{}
	assert.Equal(t, "[]", v2.String())

	var v3 psql.Vector
	assert.Equal(t, "", v3.String())
}

func TestVectorScan(t *testing.T) {
	var v psql.Vector

	// Scan from string with brackets
	err := v.Scan("[1.5,2.5,3.5]")
	require.NoError(t, err)
	assert.Equal(t, psql.Vector{1.5, 2.5, 3.5}, v)

	// Scan from []byte
	err = v.Scan([]byte("[4,5,6]"))
	require.NoError(t, err)
	assert.Equal(t, psql.Vector{4, 5, 6}, v)

	// Scan nil
	err = v.Scan(nil)
	require.NoError(t, err)
	assert.Nil(t, v)

	// Scan empty string
	err = v.Scan("")
	require.NoError(t, err)
	assert.Nil(t, v)

	// Scan empty brackets
	err = v.Scan("[]")
	require.NoError(t, err)
	assert.Equal(t, psql.Vector{}, v)

	// Scan invalid value
	err = v.Scan("[abc]")
	assert.Error(t, err)

	// Scan unsupported type
	err = v.Scan(12345)
	assert.Error(t, err)
}

func TestVectorValue(t *testing.T) {
	v := psql.Vector{1.0, 2.0, 3.0}
	val, err := v.Value()
	require.NoError(t, err)
	assert.Equal(t, "[1,2,3]", val)

	var vnil psql.Vector
	val, err = vnil.Value()
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestVectorDimensions(t *testing.T) {
	v := psql.Vector{1, 2, 3, 4, 5}
	assert.Equal(t, 5, v.Dimensions())
}

func TestVecDistanceEscapeValue(t *testing.T) {
	v := psql.Vector{1, 2, 3}

	// L2 distance
	d := psql.VecL2Distance(psql.F("Embedding"), v)
	s := d.EscapeValue()
	assert.Contains(t, s, "Embedding")
	assert.Contains(t, s, "1,2,3")

	// Cosine distance
	d2 := psql.VecCosineDistance(psql.F("Embedding"), v)
	s2 := d2.EscapeValue()
	assert.Contains(t, s2, "Embedding")

	// Inner product
	d3 := psql.VecInnerProduct(psql.F("Embedding"), v)
	s3 := d3.EscapeValue()
	assert.Contains(t, s3, "Embedding")
}

func TestVecOrderBy(t *testing.T) {
	ctx := context.Background()
	v := psql.Vector{1, 2, 3}
	query := psql.B().Select().From("items").
		OrderBy(psql.VecOrderBy(psql.F("Embedding"), v, psql.VectorL2))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "ASC")
}

func TestVecDistanceInBuilder(t *testing.T) {
	ctx := context.Background()

	v := psql.Vector{0.1, 0.2, 0.3}
	query := psql.B().Select().From("items").
		OrderBy(psql.VecL2Distance(psql.F("Embedding"), v))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "Embedding")
}

// Integration tests for vector operations

type VecTable struct {
	psql.Name `sql:"test_vector"`
	ID        int64       `sql:",key=PRIMARY"`
	Label     string      `sql:",type=VARCHAR,size=128"`
	Embedding psql.Vector `sql:",type=VECTOR,size=3"`
}

func TestVectorIntegration(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() != psql.EnginePostgreSQL {
		t.Skip("Vector tests only applicable for PostgreSQL/CockroachDB")
	}

	ctx := be.Plug(context.Background())

	// Try to enable vector support (pgvector for PostgreSQL, native for CockroachDB)
	_ = psql.Q("CREATE EXTENSION IF NOT EXISTS vector").Exec(ctx)

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_vector\"").Exec(ctx)

	// Insert vectors
	err := psql.Insert(ctx, &VecTable{ID: 1, Label: "a", Embedding: psql.Vector{1, 0, 0}})
	if err != nil {
		// Vector type not available in this environment
		t.Skipf("Vector type not supported: %v", err)
	}

	err = psql.Insert(ctx, &VecTable{ID: 2, Label: "b", Embedding: psql.Vector{0, 1, 0}})
	require.NoError(t, err)

	err = psql.Insert(ctx, &VecTable{ID: 3, Label: "c", Embedding: psql.Vector{1, 1, 0}})
	require.NoError(t, err)

	// Fetch back and verify
	obj, err := psql.Get[VecTable](ctx, map[string]any{"ID": 1})
	require.NoError(t, err)
	assert.Equal(t, "a", obj.Label)
	require.Equal(t, 3, obj.Embedding.Dimensions())
	assert.InDelta(t, float32(1), obj.Embedding[0], 0.001)
	assert.InDelta(t, float32(0), obj.Embedding[1], 0.001)
	assert.InDelta(t, float32(0), obj.Embedding[2], 0.001)

	// Count
	cnt, err := psql.Count[VecTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, cnt)

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_vector\"").Exec(ctx)
}

type VecIdxTable struct {
	psql.Name `sql:"test_vector_idx"`
	psql.Key  `sql:"EmbeddingIdx,type=VECTOR,fields='Embedding',method=hnsw"`
	ID        int64       `sql:",key=PRIMARY"`
	Label     string      `sql:",type=VARCHAR,size=128"`
	Embedding psql.Vector `sql:",type=VECTOR,size=3"`
}

func TestVectorIndexIntegration(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() != psql.EnginePostgreSQL {
		t.Skip("Vector index tests only applicable for PostgreSQL/CockroachDB")
	}

	ctx := be.Plug(context.Background())

	// Try enabling vector indexes for CockroachDB (ignore errors for standard PostgreSQL)
	_ = psql.Q("SET CLUSTER SETTING feature.vector_index.enabled = true").Exec(ctx)

	// For standard PostgreSQL, ensure pgvector extension is available
	_ = psql.Q("CREATE EXTENSION IF NOT EXISTS vector").Exec(ctx)

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_vector_idx\"").Exec(ctx)

	// Insert - this should create the table with a vector index
	err := psql.Insert(ctx, &VecIdxTable{ID: 1, Label: "x", Embedding: psql.Vector{1, 2, 3}})
	if err != nil {
		// Vector indexes might not be supported in this environment
		t.Skipf("Vector index not supported: %v", err)
	}

	err = psql.Insert(ctx, &VecIdxTable{ID: 2, Label: "y", Embedding: psql.Vector{4, 5, 6}})
	require.NoError(t, err)

	// Verify the data was inserted
	cnt, err := psql.Count[VecIdxTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_vector_idx\"").Exec(ctx)
}
