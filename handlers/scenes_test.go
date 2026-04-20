package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestScenesList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "name", "synopsis", "content", "tags", "auto_update_context",
		"position", "extra_data", "created_at", "updated_at", "concept_ids",
	}).AddRow("sc-uuid", "Opening", nil, nil, "{}", false, 0, "{}", "2026-01-01", "2026-01-01", "{}")

	mock.ExpectQuery(`SELECT s.id`).
		WithArgs("ch-uuid").
		WillReturnRows(rows)

	h := &scenesHandler{db: db}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("chapterId", "ch-uuid")
	rr := httptest.NewRecorder()
	h.list(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0]["id"] != "sc-uuid" {
		t.Fatalf("unexpected result: %v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScenesCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO scenes`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("new-sc-uuid"))
	mock.ExpectCommit()

	h := &scenesHandler{db: db}
	body := `{"name":"Opening","position":0}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.SetPathValue("chapterId", "ch-uuid")
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScenesDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM scenes`).
		WithArgs("sc-uuid", "ch-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &scenesHandler{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", "sc-uuid")
	req.SetPathValue("chapterId", "ch-uuid")
	rr := httptest.NewRecorder()
	h.delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
