package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestActsList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "position", "extra_data", "created_at", "updated_at"}).
		AddRow("act-uuid", "Act One", 0, "{}", "2026-01-01", "2026-01-01")

	mock.ExpectQuery(`SELECT id, name, position`).
		WithArgs("novel-uuid").
		WillReturnRows(rows)

	h := &actsHandler{db: db}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
	if len(result) != 1 || result[0]["id"] != "act-uuid" {
		t.Fatalf("unexpected result: %v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActsCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO acts`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("new-act-uuid"))

	h := &actsHandler{db: db}
	body := `{"name":"Act One","position":0}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.SetPathValue("novelId", "novel-uuid")
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActsDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM acts`).
		WithArgs("act-uuid", "novel-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &actsHandler{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", "act-uuid")
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
