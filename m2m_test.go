package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type M2MStudent struct {
	psql.Name `sql:"test_m2m_student"`
	ID        int64        `sql:",key=PRIMARY"`
	SName     string       `sql:",type=VARCHAR,size=128"`
	Courses   []*M2MCourse `psql:"many_to_many:test_m2m_enrollment,StudentID,CourseID"`
}

type M2MCourse struct {
	psql.Name `sql:"test_m2m_course"`
	ID        int64         `sql:",key=PRIMARY"`
	Title     string        `sql:",type=VARCHAR,size=128"`
	Students  []*M2MStudent `psql:"many_to_many:test_m2m_enrollment,CourseID,StudentID"`
}

type M2MEnrollment struct {
	psql.Name `sql:"test_m2m_enrollment"`
	StudentID int64 `sql:",key=PRIMARY"`
	CourseID  int64 `sql:",key=PRIMARY"`
}

func TestManyToMany(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_m2m_enrollment\"").Exec(ctx)
	_ = psql.Q("DROP TABLE IF EXISTS \"test_m2m_student\"").Exec(ctx)
	_ = psql.Q("DROP TABLE IF EXISTS \"test_m2m_course\"").Exec(ctx)

	// Register target tables (ensures Table[T] is called)
	_ = psql.Table[M2MStudent]()
	_ = psql.Table[M2MCourse]()
	_ = psql.Table[M2MEnrollment]()

	// Insert students
	require.NoError(t, psql.Insert(ctx, &M2MStudent{ID: 1, SName: "Alice"}))
	require.NoError(t, psql.Insert(ctx, &M2MStudent{ID: 2, SName: "Bob"}))
	require.NoError(t, psql.Insert(ctx, &M2MStudent{ID: 3, SName: "Charlie"}))

	// Insert courses
	require.NoError(t, psql.Insert(ctx, &M2MCourse{ID: 10, Title: "Math"}))
	require.NoError(t, psql.Insert(ctx, &M2MCourse{ID: 20, Title: "Science"}))
	require.NoError(t, psql.Insert(ctx, &M2MCourse{ID: 30, Title: "History"}))

	// Create enrollments (join table)
	require.NoError(t, psql.Insert(ctx, &M2MEnrollment{StudentID: 1, CourseID: 10})) // Alice -> Math
	require.NoError(t, psql.Insert(ctx, &M2MEnrollment{StudentID: 1, CourseID: 20})) // Alice -> Science
	require.NoError(t, psql.Insert(ctx, &M2MEnrollment{StudentID: 2, CourseID: 10})) // Bob -> Math
	require.NoError(t, psql.Insert(ctx, &M2MEnrollment{StudentID: 3, CourseID: 30})) // Charlie -> History

	// Preload student -> courses
	students, err := psql.Fetch[M2MStudent](ctx, nil, psql.WithPreload("Courses"))
	require.NoError(t, err)
	require.Len(t, students, 3)

	// Find Alice and verify courses
	var alice, bob, charlie *M2MStudent
	for _, s := range students {
		switch s.SName {
		case "Alice":
			alice = s
		case "Bob":
			bob = s
		case "Charlie":
			charlie = s
		}
	}

	require.NotNil(t, alice)
	assert.Len(t, alice.Courses, 2, "Alice should have 2 courses")

	require.NotNil(t, bob)
	assert.Len(t, bob.Courses, 1, "Bob should have 1 course")
	assert.Equal(t, "Math", bob.Courses[0].Title)

	require.NotNil(t, charlie)
	assert.Len(t, charlie.Courses, 1, "Charlie should have 1 course")
	assert.Equal(t, "History", charlie.Courses[0].Title)

	// Preload course -> students
	courses, err := psql.Fetch[M2MCourse](ctx, nil, psql.WithPreload("Students"))
	require.NoError(t, err)
	require.Len(t, courses, 3)

	var math *M2MCourse
	for _, c := range courses {
		if c.Title == "Math" {
			math = c
		}
	}
	require.NotNil(t, math)
	assert.Len(t, math.Students, 2, "Math should have 2 students")

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_m2m_enrollment\"").Exec(ctx)
	_ = psql.Q("DROP TABLE IF EXISTS \"test_m2m_student\"").Exec(ctx)
	_ = psql.Q("DROP TABLE IF EXISTS \"test_m2m_course\"").Exec(ctx)
}
