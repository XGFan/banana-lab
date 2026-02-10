package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router)

	request, err := http.NewRequest(http.MethodGet, "/api/health", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	expectedBody := "{\"status\":\"ok\"}"
	if recorder.Body.String() != expectedBody {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}
