package ebook_favorite

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

type favoriteSQLConnection struct {
	db.Connection
	database *sql.DB
}

func (c *favoriteSQLConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.database.Exec(query, args...)
}

func (c *favoriteSQLConnection) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := c.database.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := []map[string]interface{}{}
	for rows.Next() {
		values, targets := make([]interface{}, len(columns)), make([]interface{}, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{}, len(columns))
		for index, column := range columns {
			row[column] = values[index]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func favoriteSQLFixture(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	for _, query := range []string{
		"ATTACH DATABASE ':memory:' AS public",
		"CREATE TABLE public.ptmj_ebook_favorite (ebook_id INTEGER NOT NULL, user_id INTEGER NOT NULL, PRIMARY KEY (ebook_id, user_id))",
		"INSERT INTO public.ptmj_ebook_favorite VALUES (100, 7), (100, 8)",
	} {
		if _, err := database.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handlers := &handler{repo: newRepository(&favoriteSQLConnection{database: database})}
	group := engine.Group("/favorites")
	group.GET("", handlers.list)
	group.GET("/:id", handlers.get)
	group.POST("", handlers.create)
	group.PUT("/:id", handlers.update)
	group.DELETE("/:id", handlers.delete)
	return database, engine
}

func favoriteRequest(t *testing.T, engine *gin.Engine, method, path string, payload interface{}, status int) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("%s %s status=%d, want %d; body=%s", method, path, response.Code, status, response.Body.String())
	}
	return response
}

func requireFavoriteCount(t *testing.T, database *sql.DB, userID int64, expected int) {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT count(*) FROM public.ptmj_ebook_favorite WHERE ebook_id = 100 AND user_id = ?", userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("user %d favorite count=%d, want %d", userID, count, expected)
	}
}

func TestCompositeFavoriteCRUDPreservesOtherUsers(t *testing.T) {
	database, engine := favoriteSQLFixture(t)
	response := favoriteRequest(t, engine, http.MethodGet, "/favorites/100,7", nil, http.StatusOK)
	var detail struct {
		Data EbookFavorite `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Data.EbookId != 100 || detail.Data.UserId != 7 {
		t.Fatalf("detail=%#v", detail.Data)
	}
	// Moving one association must preserve a second user on the same resource.
	favoriteRequest(t, engine, http.MethodPut, "/favorites/100,7", EbookFavoritePayload{EbookId: 100, UserId: 9}, http.StatusOK)
	requireFavoriteCount(t, database, 7, 0)
	requireFavoriteCount(t, database, 8, 1)
	requireFavoriteCount(t, database, 9, 1)
	favoriteRequest(t, engine, http.MethodDelete, "/favorites/100,9", nil, http.StatusOK)
	requireFavoriteCount(t, database, 8, 1)
	requireFavoriteCount(t, database, 9, 0)
	favoriteRequest(t, engine, http.MethodPost, "/favorites", EbookFavoritePayload{EbookId: 100, UserId: 10}, http.StatusOK)
	requireFavoriteCount(t, database, 8, 1)
	requireFavoriteCount(t, database, 10, 1)
	favoriteRequest(t, engine, http.MethodPost, "/favorites", EbookFavoritePayload{EbookId: 100, UserId: 8}, http.StatusConflict)
	favoriteRequest(t, engine, http.MethodGet, "/favorites/100,7", nil, http.StatusNotFound)
	favoriteRequest(t, engine, http.MethodDelete, "/favorites/100,9", nil, http.StatusNotFound)
	// Existing destination pair must reject the move and preserve both rows.
	favoriteRequest(t, engine, http.MethodPut, "/favorites/100,8", EbookFavoritePayload{EbookId: 100, UserId: 10}, http.StatusConflict)
	requireFavoriteCount(t, database, 8, 1)
	requireFavoriteCount(t, database, 10, 1)
}

func TestCompositeFavoriteRejectsAmbiguousAndInvalidWrites(t *testing.T) {
	database, engine := favoriteSQLFixture(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		for _, id := range []string{"100", "100,", "100,0", "0,7", "100,7,8", "invalid,7", "100,-1"} {
			favoriteRequest(t, engine, method, "/favorites/"+id, EbookFavoritePayload{EbookId: 100, UserId: 9}, http.StatusBadRequest)
		}
	}
	for _, payload := range []map[string]int64{
		{"userId": 9}, {"ebookId": 100}, {"ebookId": 0, "userId": 9}, {"ebookId": 100, "userId": -1},
	} {
		favoriteRequest(t, engine, http.MethodPost, "/favorites", payload, http.StatusBadRequest)
	}
	favoriteRequest(t, engine, http.MethodPut, "/favorites/100,7", EbookFavoritePayload{EbookId: 101, UserId: 9}, http.StatusBadRequest)
	requireFavoriteCount(t, database, 7, 1)
	requireFavoriteCount(t, database, 8, 1)
	requireFavoriteCount(t, database, 9, 0)
}

func TestCompositeFavoritePaginationUsesBothKeys(t *testing.T) {
	_, engine := favoriteSQLFixture(t)
	first := favoriteRequest(t, engine, http.MethodGet, "/favorites?page=1&pageSize=1", nil, http.StatusOK)
	second := favoriteRequest(t, engine, http.MethodGet, "/favorites?page=2&pageSize=1", nil, http.StatusOK)
	var page1, page2 struct {
		Data EbookFavoritePage `json:"data"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if page1.Data.Total != 2 || page2.Data.Total != 2 || len(page1.Data.Items) != 1 || len(page2.Data.Items) != 1 {
		t.Fatalf("pages=%#v / %#v", page1.Data, page2.Data)
	}
	if page1.Data.Items[0].UserId != 8 || page2.Data.Items[0].UserId != 7 {
		t.Fatalf("page order did not use both keys: %#v / %#v", page1.Data.Items, page2.Data.Items)
	}
}
