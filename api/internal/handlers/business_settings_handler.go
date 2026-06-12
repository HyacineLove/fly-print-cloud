package handlers

import (
	"net/http"

	"fly-print-cloud/api/internal/business"

	"github.com/gin-gonic/gin"
)

type businessSettingsService interface {
	Current() (business.Settings, error)
	Update(settings business.Settings) (business.Settings, error)
}

type BusinessSettingsHandler struct {
	service businessSettingsService
}

func NewBusinessSettingsHandler(service businessSettingsService) *BusinessSettingsHandler {
	return &BusinessSettingsHandler{service: service}
}

func (h *BusinessSettingsHandler) Get(c *gin.Context) {
	settings, err := h.service.Current()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to load business settings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    settings,
	})
}

func (h *BusinessSettingsHandler) Update(c *gin.Context) {
	var request business.Settings
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
		})
		return
	}

	settings, err := h.service.Update(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    settings,
	})
}
