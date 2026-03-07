package psql_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StatusEnum is used for testing PostgreSQL enum fields
type StatusEnum string

const (
	StatusPending  StatusEnum = "pending"
	StatusActive   StatusEnum = "active"
	StatusInactive StatusEnum = "inactive"
	StatusDeleted  StatusEnum = "deleted"
)

// TestStructWithEnum is a test struct with an enum field
type TestStructWithEnum struct {
	psql.Name `sql:"test_enum"`
	ID        int64      `sql:",key=PRIMARY"`
	Status    StatusEnum `sql:",type=enum,values='pending,active,inactive,deleted'"`
	Title     string     `sql:",type=VARCHAR,size=255"`
}

// TestStructWithEnum2 is a test struct with an updated enum field (adding a value)
type TestStructWithEnum2 struct {
	psql.Name `sql:"test_enum"`
	ID        int64      `sql:",key=PRIMARY"`
	Status    StatusEnum `sql:",type=enum,values='pending,active,inactive,deleted,archived'"`
	Title     string     `sql:",type=VARCHAR,size=255"`
}

func TestExplicitTableName(t *testing.T) {
	// Test that explicit table names via psql.Name are used as-is
	type TestTable struct {
		psql.Name `sql:"my_explicit_name"`
		ID        int64 `sql:",key=PRIMARY"`
	}

	table := psql.Table[TestTable]()
	assert.Equal(t, "my_explicit_name", table.Name(), "Explicit table name should be preserved")

	// Test with a mock backend using LegacyNamer (which would normally transform names)
	be := &psql.Backend{}
	be.SetNamer(&psql.LegacyNamer{})

	// The formatted name should still be the explicit name, not transformed
	formattedName := table.FormattedName(be)
	assert.Equal(t, "my_explicit_name", formattedName,
		"Explicit table names should not be transformed by the namer")
}

func TestEnumTypeName(t *testing.T) {
	// Test that GetEnumTypeName generates the expected hash based on values alone
	testCases := []struct {
		values       string
		expectedHash string
		description  string
	}{
		{
			values:       "pending,active,inactive,deleted",
			expectedHash: "5ada133e", // sha256("pending|active|inactive|deleted")[:8]
			description:  "Standard enum values",
		},
		{
			values:       "pending,active,inactive,deleted,archived",
			expectedHash: "97f4445d", // sha256("pending|active|inactive|deleted|archived")[:8]
			description:  "Extended enum values",
		},
		{
			values:       "red,green,blue",
			expectedHash: "94d57770", // sha256("red|green|blue")[:8]
			description:  "Different enum values",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			expectedName := "chk_enum_" + tc.expectedHash
			actualName := psql.GetEnumTypeName(tc.values)
			assert.Equal(t, expectedName, actualName,
				"Enum type name should be based solely on values, not table/column names")
		})
	}

	// Verify that the same values always produce the same enum type name (deduplication)
	name1 := psql.GetEnumTypeName("pending,active,inactive,deleted")
	name2 := psql.GetEnumTypeName("pending,active,inactive,deleted")
	assert.Equal(t, name1, name2, "Same values should produce the same enum type name")
}

func TestEnumCreatePostgreSQL(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() != psql.EnginePostgreSQL {
		t.Skip("Test only applicable for PostgreSQL")
	}

	ctx := be.Plug(context.Background())

	// Create a table with an enum field
	obj := &TestStructWithEnum{
		ID:     1,
		Status: StatusActive,
		Title:  "Test Object",
	}

	// Insert the object (which should create the enum type and table)
	err := psql.Insert(ctx, obj)
	require.NoError(t, err, "Failed to insert object with enum field")

	// Check if the CHECK constraint was created
	// Generate deduplicated constraint name based on values hash (using pipe separator)
	hasher := sha256.New()
	hasher.Write([]byte("pending|active|inactive|deleted"))
	hash := hex.EncodeToString(hasher.Sum(nil))
	constraintName := "chk_enum_" + hash[:8]

	var constraintExists bool
	err = psql.Q("SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)", constraintName).Each(ctx, func(row *sql.Rows) error {
		return row.Scan(&constraintExists)
	})
	require.NoError(t, err, "Failed to check if CHECK constraint exists")
	assert.True(t, constraintExists, "CHECK constraint should exist")

	// Check the constraint definition
	var constraintDef string
	err = psql.Q(`
		SELECT pg_get_constraintdef(c.oid) 
		FROM pg_constraint c 
		WHERE c.conname = $1
	`, constraintName).Each(ctx, func(row *sql.Rows) error {
		return row.Scan(&constraintDef)
	})
	require.NoError(t, err, "Failed to get constraint definition")

	// Verify the constraint contains the expected values
	assert.Contains(t, constraintDef, "pending", "Constraint should contain 'pending'")
	assert.Contains(t, constraintDef, "active", "Constraint should contain 'active'")
	assert.Contains(t, constraintDef, "inactive", "Constraint should contain 'inactive'")
	assert.Contains(t, constraintDef, "deleted", "Constraint should contain 'deleted'")
	// Check for NULL handling - CockroachDB may format this differently
	assert.True(t, strings.Contains(constraintDef, "IS NULL") || strings.Contains(constraintDef, "NULL"),
		"Constraint should handle NULL values")

	// Fetch the object back
	fetchedObj, err := psql.Get[TestStructWithEnum](ctx, map[string]any{"ID": 1})
	require.NoError(t, err, "Failed to fetch object with enum field")

	// Check that values match
	assert.Equal(t, StatusActive, fetchedObj.Status, "Fetched enum value should match")
	assert.Equal(t, "Test Object", fetchedObj.Title, "Fetched title should match")

	// Clean up
	err = psql.Q("DROP TABLE IF EXISTS \"test_enum\"").Exec(ctx)
	require.NoError(t, err, "Failed to drop test table")

	// The CHECK constraint will be dropped automatically with the table
}

func TestEnumUpdatePostgreSQL(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() != psql.EnginePostgreSQL {
		t.Skip("Test only applicable for PostgreSQL")
	}

	ctx := be.Plug(context.Background())

	// Clean up any existing table from previous tests
	_ = psql.Q("DROP TABLE IF EXISTS \"test_enum\"").Exec(ctx)

	// First create a table with the original enum
	obj := &TestStructWithEnum{
		ID:     1,
		Status: StatusActive,
		Title:  "Test Object",
	}

	// Insert the object (which should create the enum type and table)
	err := psql.Insert(ctx, obj)
	require.NoError(t, err, "Failed to insert object with enum field")

	// Now try to update the enum by using a struct with updated enum values
	obj2 := &TestStructWithEnum2{
		ID:     2,
		Status: StatusActive,
		Title:  "Test Object 2",
	}

	// This should trigger an update to the enum type
	err = psql.Insert(ctx, obj2)
	require.NoError(t, err, "Failed to insert object with updated enum field")

	// Check the new CHECK constraint was created
	// Generate deduplicated constraint name based on new values hash (using pipe separator)
	hasher2 := sha256.New()
	hasher2.Write([]byte("pending|active|inactive|deleted|archived"))
	hash2 := hex.EncodeToString(hasher2.Sum(nil))
	constraintName2 := "chk_enum_" + hash2[:8]

	var constraintExists bool
	err = psql.Q("SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)", constraintName2).Each(ctx, func(row *sql.Rows) error {
		return row.Scan(&constraintExists)
	})
	require.NoError(t, err, "Failed to check if new CHECK constraint exists")
	assert.True(t, constraintExists, "New CHECK constraint should exist")

	// Check the constraint definition
	var constraintDef string
	err = psql.Q(`
		SELECT pg_get_constraintdef(c.oid) 
		FROM pg_constraint c 
		WHERE c.conname = $1
	`, constraintName2).Each(ctx, func(row *sql.Rows) error {
		return row.Scan(&constraintDef)
	})
	require.NoError(t, err, "Failed to get constraint definition")

	// Check that the constraint includes the new 'archived' value
	assert.Contains(t, constraintDef, "archived", "Constraint should contain the new 'archived' value")

	// Clean up
	err = psql.Q("DROP TABLE IF EXISTS \"test_enum\"").Exec(ctx)
	require.NoError(t, err, "Failed to drop test table")

	// The CHECK constraints will be dropped automatically with the table
}
