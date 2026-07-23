package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fly-print-cloud/api/internal/business"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

type fixedOpsSettings struct {
	max int
}

func (s fixedOpsSettings) Current() (business.Settings, error) {
	return business.Settings{MaxContactsPerNode: s.max}, nil
}

func TestOpsContactHandlerListSelfContacts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	repo := database.NewOpsContactRepository(&database.DB{DB: sqlDB})
	handler := NewOpsContactHandler(repo, fixedOpsSettings{max: 5})

	mock.ExpectQuery(`SELECT c.name, c.phone`).
		WithArgs("node-1").
		WillReturnRows(sqlmock.NewRows([]string{"name", "phone"}).AddRow("张三", "13800000000"))

	router := gin.New()
	router.GET("/self/contacts", func(c *gin.Context) {
		c.Set("node_id", "node-1")
		handler.ListSelfContacts(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/self/contacts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code int                       `json:"code"`
		Data []models.OpsContactPublic `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 200 || len(body.Data) != 1 || body.Data[0].Name != "张三" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestOpsContactHandlerCreateValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	handler := NewOpsContactHandler(database.NewOpsContactRepository(&database.DB{DB: sqlDB}), fixedOpsSettings{max: 5})
	router := gin.New()
	router.POST("/ops-contacts", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/ops-contacts", bytes.NewBufferString(`{"name":"","phone":"138"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
