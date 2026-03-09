package main

import (
	"testing"

	"fly-print-cloud/api/internal/handlers"
	"fly-print-cloud/api/internal/websocket"

	"github.com/gin-gonic/gin"
)

func TestSetupRoutes_RegistersFilePreflightRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	setupRoutes(
		router,
		&handlers.UserHandler{},
		&handlers.EdgeNodeHandler{},
		&handlers.PrinterHandler{},
		&handlers.PrintJobHandler{},
		&websocket.WebSocketHandler{},
		&handlers.OAuth2Handler{},
		&handlers.FileHandler{},
		&handlers.HealthHandler{},
		nil,
		nil,
		nil,
		nil,
	)

	found := false
	for _, route := range router.Routes() {
		if route.Method == "POST" && route.Path == "/api/v1/files/preflight" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("route /api/v1/files/preflight not registered")
	}
}
