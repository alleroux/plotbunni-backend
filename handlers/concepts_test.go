package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestConceptsList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "type", "name", "aliases", "tags", "description", "notes",
		"priority", "image", "extra_data", "created_at", "updated_at",
	}).AddRow("c-uuid", "character", "Aria", "{}", "{}", nil, nil, 0, nil, "{}", "2026-01-01", "2026-01-01")

	mock.ExpectQuery(`SELECT id, type, name`).
		WithArgs("novel-uuid").
		WillReturnRows(rows)

	h := &conceptsHandler{db: db}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/novels/novel-uuid/concepts", nil)
	req.SetPathValue("novelId", "novel-uuid")
	rr := httptest.NewRecorder()
	h.list(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0]["id"] != "c-uuid" {
		t.Fatalf("unexpected result: %v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConceptsCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO concepts`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("new-concept-uuid"))

	h := &conceptsHandler{db: db}
	body := `{"type":"character","name":"Aria"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/novels/novel-uuid/concepts", strings.NewReader(body))
	req.SetPathValue("novelId", "novel-uuid")
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != "new-concept-uuid" {
		t.Fatalf("unexpected id: %v", result["id"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConceptsDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM concepts`).
		WithArgs("c-uuid", "novel-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &conceptsHandler{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", "c-uuid")
	req.SetPathValue("novelId", "novel-uuid")
	rr := httptest.NewRecorder()
	h.delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
