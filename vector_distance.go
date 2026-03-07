package psql

import (
	"fmt"
	"strings"
)

// VectorDistanceOp represents the type of vector distance operation.
type VectorDistanceOp int

const (
	// VectorL2 is the L2 (Euclidean) distance operator.
	// PostgreSQL pgvector: <->   CockroachDB: vec_l2_distance()
	VectorL2 VectorDistanceOp = iota
	// VectorCosine is the cosine distance operator.
	// PostgreSQL pgvector: <=>   CockroachDB: vec_cosine_distance()
	VectorCosine
	// VectorInnerProduct is the negative inner product operator.
	// PostgreSQL pgvector: <#>   CockroachDB: vec_inner_product()
	VectorInnerProduct
)

// VectorDistance represents a vector distance calculation between a field and a vector.
// It implements EscapeValueable and SortValueable so it can be used in WHERE and ORDER BY.
//
// Usage:
//
//	// In ORDER BY (nearest neighbor search):
//	psql.B().Select().From("items").OrderBy(psql.VecL2Distance(psql.F("Embedding"), queryVec))
//
//	// In WHERE (filter by distance):
//	psql.B().Select().From("items").Where(map[string]any{
//	    psql.VecCosineDistance(psql.F("Embedding"), queryVec).String(): psql.Lt(nil, 0.5),
//	})
type VectorDistance struct {
	Field any              // typically a fieldName via psql.F()
	Vec   Vector           // the query vector
	Op    VectorDistanceOp // distance type
}

// VecL2Distance creates an L2 (Euclidean) distance expression.
func VecL2Distance(field any, vec Vector) *VectorDistance {
	return &VectorDistance{Field: field, Vec: vec, Op: VectorL2}
}

// VecCosineDistance creates a cosine distance expression.
func VecCosineDistance(field any, vec Vector) *VectorDistance {
	return &VectorDistance{Field: field, Vec: vec, Op: VectorCosine}
}

// VecInnerProduct creates a negative inner product distance expression.
func VecInnerProduct(field any, vec Vector) *VectorDistance {
	return &VectorDistance{Field: field, Vec: vec, Op: VectorInnerProduct}
}

// String returns a display representation (not engine-specific).
func (d *VectorDistance) String() string {
	return d.EscapeValue()
}

// EscapeValue renders the distance expression without engine context (defaults to function syntax).
func (d *VectorDistance) EscapeValue() string {
	return d.renderFunc(nil)
}

// escapeValueCtx renders the distance expression with engine-specific syntax.
func (d *VectorDistance) escapeValueCtx(ctx *renderContext) string {
	if ctx != nil && ctx.e == EnginePostgreSQL {
		return d.renderPG(ctx)
	}
	return d.renderFunc(ctx)
}

// sortEscapeValue implements SortValueable for use in ORDER BY.
func (d *VectorDistance) sortEscapeValue() string {
	return d.EscapeValue()
}

// renderPG renders using PostgreSQL pgvector operator syntax.
// pgvector operators: <-> (L2), <=> (cosine), <#> (inner product)
// CockroachDB also supports these operators for its native vector type.
func (d *VectorDistance) renderPG(ctx *renderContext) string {
	b := &strings.Builder{}
	b.WriteString(escapeCtx(ctx, d.Field))

	switch d.Op {
	case VectorL2:
		b.WriteString(" <-> ")
	case VectorCosine:
		b.WriteString(" <=> ")
	case VectorInnerProduct:
		b.WriteString(" <#> ")
	}

	b.WriteString(escapeCtx(ctx, d.Vec.String()))
	return b.String()
}

// renderFunc renders using function call syntax (fallback for non-PostgreSQL engines).
func (d *VectorDistance) renderFunc(ctx *renderContext) string {
	b := &strings.Builder{}

	var funcName string
	switch d.Op {
	case VectorL2:
		funcName = "vec_l2_distance"
	case VectorCosine:
		funcName = "vec_cosine_distance"
	case VectorInnerProduct:
		funcName = "vec_inner_product"
	default:
		funcName = "vec_l2_distance"
	}

	b.WriteString(funcName)
	b.WriteByte('(')
	if ctx != nil {
		b.WriteString(escapeCtx(ctx, d.Field))
		b.WriteString(", ")
		b.WriteString(escapeCtx(ctx, d.Vec.String()))
	} else {
		b.WriteString(Escape(d.Field))
		b.WriteString(", ")
		b.WriteString(Escape(d.Vec.String()))
	}
	b.WriteByte(')')

	return b.String()
}

// VecOrderBy is a convenience that returns a SortValueable for ordering by vector distance.
// The direction defaults to ASC (nearest first).
//
// Usage:
//
//	psql.B().Select().From("items").OrderBy(psql.VecOrderBy(psql.F("Embedding"), queryVec, psql.VectorL2))
func VecOrderBy(field any, vec Vector, op VectorDistanceOp) SortValueable {
	return &ordVecDistance{
		dist: &VectorDistance{Field: field, Vec: vec, Op: op},
		ord:  "ASC",
	}
}

type ordVecDistance struct {
	dist *VectorDistance
	ord  string
}

func (o *ordVecDistance) sortEscapeValue() string {
	return fmt.Sprintf("%s %s", o.dist.EscapeValue(), o.ord)
}
