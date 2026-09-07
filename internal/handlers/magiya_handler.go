package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// MagiyaHandler handles HTTP requests for Magiya endpoints
type MagiyaHandler struct {
	service *services.MagiyaService
	logger  *logrus.Logger
}

// NewMagiyaHandler creates a new Magiya handler
func NewMagiyaHandler(service *services.MagiyaService, logger *logrus.Logger) *MagiyaHandler {
	return &MagiyaHandler{
		service: service,
		logger:  logger,
	}
}

// GetStations handles GET /api/v1/magiya/stations
// @Summary Get Magiya stations
// @Description Get cached Magiya stations list
// @Tags Magiya
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/magiya/stations [get]
func (h *MagiyaHandler) GetStations(c *gin.Context) {
	stationsJSON, err := h.service.GetStations()
	if err != nil {
		h.logger.WithError(err).Error("Failed to provide Magiya stations")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to load stations data",
		})
		return
	}

	// Because stationsJSON is exactly the raw JSON array from Magiya,
	// we just write it exactly to the response without decoding/encoding.
	c.Data(http.StatusOK, "application/json; charset=utf-8", stationsJSON)
}

// GetSchedules handles GET /api/v1/magiya/schedules
// @Summary Get Magiya schedules
// @Description Secure proxy for Magiya get-schedules API
// @Tags Magiya
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/magiya/schedules [get]
func (h *MagiyaHandler) GetSchedules(c *gin.Context) {
	rawQuery := c.Request.URL.RawQuery
	
	schedulesJSON, err := h.service.GetSchedules(rawQuery)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch Magiya schedules proxy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to load schedules data",
		})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", schedulesJSON)
}

// GetSeatLayout
// @Summary Get Magiya seat layout
// @Description Secure proxy for Magiya get-seat-map API
// @Tags Magiya
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/magiya/seat-map [get]
func (h *MagiyaHandler) GetSeatLayout(c *gin.Context) {
	rawQuery := c.Request.URL.RawQuery
	
	seatMapJSON, err := h.service.GetSeatLayout(rawQuery)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch Magiya seat-map proxy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to load seat layout",
		})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", seatMapJSON)
}
