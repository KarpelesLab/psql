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

func TestVecEqualInBuilder(t *testing.T) {
	ctx := context.Background()
	v := psql.Vector{1, 2, 3}

	// VecEqual
	query := psql.B().Select().From("items").
		Where(psql.VecEqual(psql.F("Embedding"), v))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "= ")
	assert.Contains(t, sql, "Embedding")
	assert.Contains(t, sql, "1,2,3")

	// VecNotEqual
	query2 := psql.B().Select().From("items").
		Where(psql.VecNotEqual(psql.F("Embedding"), v))
	sql2, err := query2.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql2, "<>")
	assert.Contains(t, sql2, "Embedding")
}

func TestVecDistanceWithThreshold(t *testing.T) {
	ctx := context.Background()
	v := psql.Vector{1, 2, 3}

	// Use distance in WHERE with a threshold
	query := psql.B().Select().From("items").
		Where(psql.Lt(psql.VecCosineDistance(psql.F("Embedding"), v), 0.5))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE")
	assert.Contains(t, sql, "Embedding")
	assert.Contains(t, sql, "0.5")
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

func TestVectorOperatorsIntegration(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() != psql.EnginePostgreSQL {
		t.Skip("Vector operator tests only applicable for PostgreSQL/CockroachDB")
	}

	ctx := be.Plug(context.Background())

	_ = psql.Q("CREATE EXTENSION IF NOT EXISTS vector").Exec(ctx)
	_ = psql.Q("DROP TABLE IF EXISTS \"test_vector_ops\"").Exec(ctx)

	type VecOpsTable struct {
		psql.Name `sql:"test_vector_ops"`
		ID        int64       `sql:",key=PRIMARY"`
		Label     string      `sql:",type=VARCHAR,size=128"`
		Embedding psql.Vector `sql:",type=VECTOR,size=3"`
	}

	// Insert test vectors
	err := psql.Insert(ctx, &VecOpsTable{ID: 1, Label: "origin_x", Embedding: psql.Vector{1, 0, 0}})
	if err != nil {
		t.Skipf("Vector type not supported: %v", err)
	}
	err = psql.Insert(ctx, &VecOpsTable{ID: 2, Label: "origin_y", Embedding: psql.Vector{0, 1, 0}})
	require.NoError(t, err)
	err = psql.Insert(ctx, &VecOpsTable{ID: 3, Label: "diagonal", Embedding: psql.Vector{1, 1, 0}})
	require.NoError(t, err)
	err = psql.Insert(ctx, &VecOpsTable{ID: 4, Label: "far", Embedding: psql.Vector{10, 10, 10}})
	require.NoError(t, err)

	t.Run("L2Distance_OrderBy", func(t *testing.T) {
		// Order by L2 distance from [1,0,0] — closest should be "origin_x" (distance 0)
		queryVec := psql.Vector{1, 0, 0}
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			OrderBy(psql.VecL2Distance(psql.F("Embedding"), queryVec)).
			Limit(4).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		require.Len(t, labels, 4)
		assert.Equal(t, "origin_x", labels[0], "closest vector should be origin_x")
		assert.Equal(t, "far", labels[3], "farthest vector should be far")
	})

	t.Run("CosineDistance_OrderBy", func(t *testing.T) {
		// Order by cosine distance from [1,1,0] — "diagonal" should be closest (same direction)
		queryVec := psql.Vector{1, 1, 0}
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			OrderBy(psql.VecCosineDistance(psql.F("Embedding"), queryVec)).
			Limit(4).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		require.Len(t, labels, 4)
		assert.Equal(t, "diagonal", labels[0], "closest by cosine should be diagonal")
	})

	t.Run("InnerProduct_OrderBy", func(t *testing.T) {
		// Order by negative inner product from [1,0,0]
		// inner product: origin_x=1, origin_y=0, diagonal=1, far=10
		// negative inner product (ascending): origin_y(0), origin_x(1), diagonal(1), far(10)
		// Note: <#> returns negative inner product, so ascending = highest inner product last
		queryVec := psql.Vector{1, 0, 0}
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			OrderBy(psql.VecOrderBy(psql.F("Embedding"), queryVec, psql.VectorInnerProduct)).
			Limit(4).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		require.Len(t, labels, 4)
		// Negative inner product ascending means smallest (most negative) first = highest actual IP
		// For <#>: -IP, so origin_y has -0=0, origin_x has -1, diagonal has -1, far has -10
		// Ascending: far(-10), origin_x(-1) or diagonal(-1), origin_y(0)
		assert.Equal(t, "far", labels[0], "highest inner product first with ASC on negative IP")
	})

	t.Run("VecEqual_Where", func(t *testing.T) {
		// Find the exact vector [1,0,0]
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			Where(psql.VecEqual(psql.F("Embedding"), psql.Vector{1, 0, 0})).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		require.Len(t, labels, 1)
		assert.Equal(t, "origin_x", labels[0])
	})

	t.Run("VecNotEqual_Where", func(t *testing.T) {
		// Find all vectors != [1,0,0]
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			Where(psql.VecNotEqual(psql.F("Embedding"), psql.Vector{1, 0, 0})).
			OrderBy(psql.S("ID")).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		require.Len(t, labels, 3)
		assert.Equal(t, "origin_y", labels[0])
		assert.Equal(t, "diagonal", labels[1])
		assert.Equal(t, "far", labels[2])
	})

	t.Run("L2Distance_Where_Threshold", func(t *testing.T) {
		// Find vectors within L2 distance < 2.0 of [1,0,0]
		// L2 distances from [1,0,0]: origin_x=0, origin_y=sqrt(2)≈1.414, diagonal=1, far=sqrt(181)≈13.45
		queryVec := psql.Vector{1, 0, 0}
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			Where(psql.Lt(psql.VecL2Distance(psql.F("Embedding"), queryVec), 2.0)).
			OrderBy(psql.VecL2Distance(psql.F("Embedding"), queryVec)).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		require.Len(t, labels, 3, "origin_x(0), diagonal(1), origin_y(1.414) should be within distance 2")
		assert.Equal(t, "origin_x", labels[0])
		assert.Equal(t, "diagonal", labels[1])
		assert.Equal(t, "origin_y", labels[2])
	})

	t.Run("CosineDistance_Where_Threshold", func(t *testing.T) {
		// Find vectors within cosine distance < 0.5 of [1,1,0]
		// Cosine distance = 1 - cosine_similarity
		// diagonal [1,1,0]: cos_sim=1, dist=0
		// origin_x [1,0,0]: cos_sim=1/sqrt(2)≈0.707, dist≈0.293
		// origin_y [0,1,0]: cos_sim=1/sqrt(2)≈0.707, dist≈0.293
		// far [10,10,10]: cos_sim=20/(sqrt(2)*sqrt(300))≈0.816, dist≈0.184
		queryVec := psql.Vector{1, 1, 0}
		rows, err := psql.B().
			Select("ID", "Label").
			From("test_vector_ops").
			Where(psql.Lt(psql.VecCosineDistance(psql.F("Embedding"), queryVec), 0.5)).
			OrderBy(psql.VecCosineDistance(psql.F("Embedding"), queryVec)).
			RunQuery(ctx)
		require.NoError(t, err)
		defer rows.Close()

		var labels []string
		for rows.Next() {
			var id int64
			var label string
			require.NoError(t, rows.Scan(&id, &label))
			labels = append(labels, label)
		}
		require.NoError(t, rows.Err())
		assert.GreaterOrEqual(t, len(labels), 1, "at least diagonal should be within cosine distance 0.5")
		assert.Equal(t, "diagonal", labels[0], "diagonal should be most similar")
	})

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_vector_ops\"").Exec(ctx)
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
