package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestChaptersList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "position", "extra_data", "created_at", "updated_at"}).
		AddRow("ch-uuid", "Chapter 1", 0, "{}", "2026-01-01", "2026-01-01")

	mock.ExpectQuery(`SELECT id, name, position`).
		WithArgs("act-uuid").
		WillReturnRows(rows)

	h := &chaptersHandler{db: db}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("actId", "act-uuid")
	rr := httptest.NewRecorder()
	h.list(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0]["id"] != "ch-uuid" {
		t.Fatalf("unexpected result: %v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChaptersCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO chapters`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("new-ch-uuid"))

	h := &chaptersHandler{db: db}
	body := `{"name":"Chapter 1","position":0}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.SetPathValue("actId", "act-uuid")
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChaptersDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM chapters`).
		WithArgs("ch-uuid", "act-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &chaptersHandler{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", "ch-uuid")
	req.SetPathValue("actId", "act-uuid")
	rr := httptest.NewRecorder()
	h.delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
