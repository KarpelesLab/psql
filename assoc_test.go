package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type AssocAuthor struct {
	psql.Name  `sql:"test_assoc_author"`
	ID         int64        `sql:",key=PRIMARY"`
	AuthorName string       `sql:",type=VARCHAR,size=128"`
	Books      []*AssocBook `psql:"has_many:AuthorID"`
}

type AssocBook struct {
	psql.Name `sql:"test_assoc_book"`
	ID        int64        `sql:",key=PRIMARY"`
	AuthorID  int64        `sql:",type=BIGINT"`
	Title     string       `sql:",type=VARCHAR,size=256"`
	Author    *AssocAuthor `psql:"belongs_to:AuthorID"`
}

type AssocProfile struct {
	psql.Name `sql:"test_assoc_profile"`
	ID        int64  `sql:",key=PRIMARY"`
	AuthorID  int64  `sql:",type=BIGINT"`
	Bio       string `sql:",type=VARCHAR,size=512"`
}

type AssocAuthorWithProfile struct {
	psql.Name  `sql:"test_assoc_author"`
	ID         int64         `sql:",key=PRIMARY"`
	AuthorName string        `sql:",type=VARCHAR,size=128"`
	Profile    *AssocProfile `psql:"has_one:AuthorID"`
}

func setupAssocTables(t *testing.T) (context.Context, func()) {
	t.Helper()
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	// Drop tables (order matters for foreign keys)
	_ = psql.Q(`DROP TABLE IF EXISTS "test_assoc_profile"`).Exec(ctx)
	_ = psql.Q(`DROP TABLE IF EXISTS "test_assoc_book"`).Exec(ctx)
	_ = psql.Q(`DROP TABLE IF EXISTS "test_assoc_author"`).Exec(ctx)

	// Ensure tables are registered
	_ = psql.Table[AssocAuthor]()
	_ = psql.Table[AssocBook]()
	_ = psql.Table[AssocProfile]()
	_ = psql.Table[AssocAuthorWithProfile]()

	cleanup := func() {
		_ = psql.Q(`DROP TABLE IF EXISTS "test_assoc_profile"`).Exec(ctx)
		_ = psql.Q(`DROP TABLE IF EXISTS "test_assoc_book"`).Exec(ctx)
		_ = psql.Q(`DROP TABLE IF EXISTS "test_assoc_author"`).Exec(ctx)
	}

	return ctx, cleanup
}

func TestAssocBelongsTo(t *testing.T) {
	ctx, cleanup := setupAssocTables(t)
	defer cleanup()

	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 1, AuthorName: "Alice"}))
	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 2, AuthorName: "Bob"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 1, AuthorID: 1, Title: "Book A"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 2, AuthorID: 1, Title: "Book B"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 3, AuthorID: 2, Title: "Book C"}))

	books, err := psql.Fetch[AssocBook](ctx, nil)
	require.NoError(t, err)
	require.Len(t, books, 3)

	err = psql.Preload(ctx, books, "Author")
	require.NoError(t, err)

	for _, book := range books {
		require.NotNil(t, book.Author, "Author should be loaded for book %d", book.ID)
		if book.AuthorID == 1 {
			assert.Equal(t, "Alice", book.Author.AuthorName)
		} else {
			assert.Equal(t, "Bob", book.Author.AuthorName)
		}
	}
}

func TestAssocHasMany(t *testing.T) {
	ctx, cleanup := setupAssocTables(t)
	defer cleanup()

	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 1, AuthorName: "Alice"}))
	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 2, AuthorName: "Bob"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 1, AuthorID: 1, Title: "Book A"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 2, AuthorID: 1, Title: "Book B"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 3, AuthorID: 2, Title: "Book C"}))

	authors, err := psql.Fetch[AssocAuthor](ctx, nil)
	require.NoError(t, err)
	require.Len(t, authors, 2)

	err = psql.Preload(ctx, authors, "Books")
	require.NoError(t, err)

	for _, author := range authors {
		require.NotNil(t, author.Books)
		if author.ID == 1 {
			assert.Len(t, author.Books, 2)
		} else {
			assert.Len(t, author.Books, 1)
			assert.Equal(t, "Book C", author.Books[0].Title)
		}
	}
}

func TestAssocHasOne(t *testing.T) {
	ctx, cleanup := setupAssocTables(t)
	defer cleanup()

	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 1, AuthorName: "Alice"}))
	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 2, AuthorName: "Bob"}))
	require.NoError(t, psql.Insert(ctx, &AssocProfile{ID: 1, AuthorID: 1, Bio: "Writes fiction"}))
	require.NoError(t, psql.Insert(ctx, &AssocProfile{ID: 2, AuthorID: 2, Bio: "Writes non-fiction"}))

	authors, err := psql.Fetch[AssocAuthorWithProfile](ctx, nil)
	require.NoError(t, err)
	require.Len(t, authors, 2)

	err = psql.Preload(ctx, authors, "Profile")
	require.NoError(t, err)

	for _, author := range authors {
		require.NotNil(t, author.Profile, "Profile should be loaded for author %d", author.ID)
		if author.ID == 1 {
			assert.Equal(t, "Writes fiction", author.Profile.Bio)
		} else {
			assert.Equal(t, "Writes non-fiction", author.Profile.Bio)
		}
	}
}

func TestAssocWithPreloadOption(t *testing.T) {
	ctx, cleanup := setupAssocTables(t)
	defer cleanup()

	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 1, AuthorName: "Alice"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 1, AuthorID: 1, Title: "Book A"}))

	// Use WithPreload in FetchOptions
	books, err := psql.Fetch[AssocBook](ctx, nil, psql.WithPreload("Author"))
	require.NoError(t, err)
	require.Len(t, books, 1)
	require.NotNil(t, books[0].Author)
	assert.Equal(t, "Alice", books[0].Author.AuthorName)
}

func TestAssocGetWithPreload(t *testing.T) {
	ctx, cleanup := setupAssocTables(t)
	defer cleanup()

	require.NoError(t, psql.Insert(ctx, &AssocAuthor{ID: 1, AuthorName: "Alice"}))
	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 1, AuthorID: 1, Title: "Book A"}))

	// Use WithPreload with Get
	book, err := psql.Get[AssocBook](ctx, map[string]any{"ID": int64(1)}, psql.WithPreload("Author"))
	require.NoError(t, err)
	require.NotNil(t, book.Author)
	assert.Equal(t, "Alice", book.Author.AuthorName)
}

func TestAssocPreloadEmpty(t *testing.T) {
	// Preload with empty targets should not error
	err := psql.Preload[AssocBook](context.Background(), nil, "Author")
	assert.NoError(t, err)
}

func TestAssocUnknownField(t *testing.T) {
	ctx, cleanup := setupAssocTables(t)
	defer cleanup()

	require.NoError(t, psql.Insert(ctx, &AssocBook{ID: 1, AuthorID: 1, Title: "Book A"}))

	books, err := psql.Fetch[AssocBook](ctx, nil)
	require.NoError(t, err)

	err = psql.Preload(ctx, books, "NonExistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown association")
}
