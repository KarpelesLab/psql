package psql_test

import (
	"strings"
	"testing"

	"github.com/portablesql/psql"
	"github.com/stretchr/testify/assert"
)

// TestEnumConstraintGeneration tests the CHECK constraint SQL generation
func TestEnumConstraintGeneration(t *testing.T) {
	// Test struct with multiple enum fields having same values
	type TestTableMultiEnum struct {
		psql.Name `sql:"test_multi_enum"`
		ID        int64  `sql:",key=PRIMARY"`
		Status    string `sql:",type=enum,values='pending,active,inactive,deleted'"`
		Priority  string `sql:",type=enum,values='pending,active,inactive,deleted'"`
		Category  string `sql:",type=enum,values='red,green,blue'"`
	}

	// Get the table metadata
	table := psql.Table[TestTableMultiEnum]()

	// Note: We can't easily create a mock Backend without a database connection,
	// but we can test the constraint name generation independently

	// Test that constraint names are generated correctly
	constraint1 := psql.GetEnumConstraintName("pending,active,inactive,deleted")
	assert.Equal(t, "chk_enum_5ada133e", constraint1, "Constraint name should be based on hash of pipe-separated values")

	constraint2 := psql.GetEnumConstraintName("red,green,blue")
	assert.Equal(t, "chk_enum_94d57770", constraint2, "Different values should produce different constraint name")

	// Verify that the same values always produce the same constraint name (deduplication)
	constraint1_again := psql.GetEnumConstraintName("pending,active,inactive,deleted")
	assert.Equal(t, constraint1, constraint1_again, "Same values should always produce same constraint name")

	// Test the table name
	assert.Equal(t, "test_multi_enum", table.Name(), "Explicit table names should be preserved")
}

// TestEnumConstraintSQL tests the actual SQL generation for CHECK constraints
func TestEnumConstraintSQL(t *testing.T) {
	// Create a mock EnumConstraint
	constraint := &psql.EnumConstraint{
		Name:   "chk_enum_5ada133e",
		Values: []string{"pending", "active", "inactive", "deleted"},
		Columns: map[string][]string{
			"test_table": {"status", "priority"},
		},
	}

	// Generate the CHECK SQL
	checkSQL := psql.GenerateEnumCheckSQL(constraint, "test_table")

	// Verify the SQL contains proper NULL handling and all values
	assert.Contains(t, checkSQL, "CONSTRAINT", "Should have CONSTRAINT keyword")
	assert.Contains(t, checkSQL, "chk_enum_5ada133e", "Should include constraint name")
	assert.Contains(t, checkSQL, "CHECK", "Should have CHECK keyword")
	assert.Contains(t, checkSQL, "IS NULL OR", "Should handle NULL values")
	assert.Contains(t, checkSQL, "'pending'", "Should include enum value 'pending'")
	assert.Contains(t, checkSQL, "'active'", "Should include enum value 'active'")
	assert.Contains(t, checkSQL, "'inactive'", "Should include enum value 'inactive'")
	assert.Contains(t, checkSQL, "'deleted'", "Should include enum value 'deleted'")

	// Should have checks for both columns
	assert.Contains(t, checkSQL, "status", "Should check status column")
	assert.Contains(t, checkSQL, "priority", "Should check priority column")

	// Verify AND logic for multiple columns
	assert.Contains(t, checkSQL, " AND ", "Multiple columns should be combined with AND")

	// Check the structure matches expected format
	// Expected: CONSTRAINT "chk_enum_5ada133e" CHECK (("status" IS NULL OR "status" IN (...)) AND ("priority" IS NULL OR "priority" IN (...)))
	assert.True(t, strings.Contains(checkSQL, "status") && strings.Contains(checkSQL, "priority"),
		"Both columns should be included in the constraint")
}

// TestEnumConstraintWithSpecialChars tests handling of special characters in enum values
func TestEnumConstraintWithSpecialChars(t *testing.T) {
	// Test with values containing special characters
	values := "value's,value\"with\"quotes,value-with-dash"
	constraintName := psql.GetEnumConstraintName(values)

	// Should still generate a valid constraint name
	assert.True(t, strings.HasPrefix(constraintName, "chk_enum_"), "Constraint name should have correct prefix")
	assert.Len(t, constraintName, 17, "Constraint name should be 17 chars (chk_enum_ + 8 hash chars)")

	// Test SQL generation with special characters
	constraint := &psql.EnumConstraint{
		Name:   constraintName,
		Values: []string{"value's", "value\"with\"quotes", "value-with-dash"},
		Columns: map[string][]string{
			"test_table": {"status"},
		},
	}

	checkSQL := psql.GenerateEnumCheckSQL(constraint, "test_table")

	// Should properly escape the values
	assert.Contains(t, checkSQL, "''", "Single quotes should be escaped")
	assert.NotContains(t, checkSQL, "value's", "Unescaped single quote should not appear")
}
