package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestNovelsList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "synopsis", "cover_image", "author", "updated_at"}).
		AddRow("uuid-1", "My Novel", nil, nil, nil, "2026-01-01T00:00:00Z")

	mock.ExpectQuery(`SELECT id, name, synopsis, cover_image, author, updated_at FROM novels`).
		WillReturnRows(rows)

	h := &novelsHandler{db: db}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/novels", nil)
	rr := httptest.NewRecorder()
	h.list(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var result []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 novel, got %d", len(result))
	}
	if result[0]["id"] != "uuid-1" {
		t.Fatalf("unexpected id: %v", result[0]["id"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNovelsCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO novels`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("new-uuid"))

	h := &novelsHandler{db: db}
	body := `{"name":"New Novel"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/novels", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != "new-uuid" {
		t.Fatalf("unexpected id: %v", result["id"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNovelsCreate_badJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &novelsHandler{db: db}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/novels", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestNovelsDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM novels`).
		WithArgs("uuid-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &novelsHandler{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/novels/uuid-1", nil)
	req.SetPathValue("id", "uuid-1")
	rr := httptest.NewRecorder()
	h.delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
