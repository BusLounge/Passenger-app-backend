package handlers

import (
	"net/http"
	"strconv"

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

// SearchSchedules handles GET /api/v1/magiya/search
// @Summary Search Magiya schedules using origin and destination coordinates
// @Description Matches coordinates to nearest Magiya stations using expanding radius (1km, 3km, etc.) and returns schedules
// @Tags Magiya
// @Produce json
// @Param from_lat query number true "Origin Latitude"
// @Param from_lng query number true "Origin Longitude"
// @Param to_lat query number true "Destination Latitude"
// @Param to_lng query number true "Destination Longitude"
// @Param date query string false "Travel Date (YYYY-MM-DD)"
// @Success 200 {object} services.MagiyaCoordinateSearchResult
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/magiya/search [get]
func (h *MagiyaHandler) SearchSchedules(c *gin.Context) {
	fromLatStr := c.Query("from_lat")
	fromLngStr := c.Query("from_lng")
	toLatStr := c.Query("to_lat")
	toLngStr := c.Query("to_lng")
	date := c.Query("date")

	fromLat, err1 := strconv.ParseFloat(fromLatStr, 64)
	fromLng, err2 := strconv.ParseFloat(fromLngStr, 64)
	toLat, err3 := strconv.ParseFloat(toLatStr, 64)
	toLng, err4 := strconv.ParseFloat(toLngStr, 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "from_lat, from_lng, to_lat, and to_lng are required numeric parameters",
		})
		return
	}

	result, err := h.service.SearchSchedulesByCoordinates(fromLat, fromLng, toLat, toLng, date)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search Magiya schedules by coordinates")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to search Magiya schedules",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
