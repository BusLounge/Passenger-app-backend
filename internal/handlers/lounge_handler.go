package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeHandler handles lounge-related HTTP requests
type LoungeHandler struct {
	loungeRepo      *database.LoungeRepository
	loungeOwnerRepo *database.LoungeOwnerRepository
	loungeRouteRepo *database.LoungeRouteRepository
}

// NewLoungeHandler creates a new lounge handler
func NewLoungeHandler(
	loungeRepo *database.LoungeRepository,
	loungeOwnerRepo *database.LoungeOwnerRepository,
	loungeRouteRepo *database.LoungeRouteRepository,
) *LoungeHandler {
	return &LoungeHandler{
		loungeRepo:      loungeRepo,
		loungeOwnerRepo: loungeOwnerRepo,
		loungeRouteRepo: loungeRouteRepo,
	}
}

// ===================================================================
// ADD LOUNGE (STEP 3: Registration)
// ===================================================================

// AddLoungeRequest represents the lounge creation request
type AddLoungeRequest struct {
	LoungeName    string   `json:"lounge_name" binding:"required"`
	Address       string   `json:"address" binding:"required"`
	ContactPhone  string   `json:"contact_phone" binding:"required"`
	Latitude      *string  `json:"latitude" binding:"required"`  // Required for map location
	Longitude     *string  `json:"longitude" binding:"required"` // Required for map location
	Capacity      *int     `json:"capacity"`                     // Maximum number of people
	Price1Hour    *string  `json:"price_1_hour"`                 // DECIMAL as string (e.g., "500.00")
	Price2Hours   *string  `json:"price_2_hours"`                // DECIMAL as string (e.g., "900.00")
	Price3Hours   *string  `json:"price_3_hours"`                // DECIMAL as string (e.g., "1200.00")
	PriceUntilBus *string  `json:"price_until_bus"`              // DECIMAL as string (e.g., "1500.00")
	Amenities     []string `json:"amenities"`                    // Array: ["wifi", "ac", "cafeteria", "charging_ports", "entertainment", "parking", "restrooms", "waiting_area"]
	Images        []string `json:"images"`                       // Array of image URLs
	// Routes that the lounge serves (array of route-stop combinations)
	Routes []models.LoungeRouteRequest `json:"routes" binding:"required,min=1"` // At least one route required
}

// AddLounge handles POST /api/v1/lounge-owner/register/add-lounge
func (h *LoungeHandler) AddLounge(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req AddLoungeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ERROR: Failed to bind request body for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	log.Printf("INFO: Add lounge request received - User: %s, Lounge: %s, Capacity: %v, Photos: %d, Routes: %d",
		userCtx.UserID, req.LoungeName, req.Capacity, len(req.Images), len(req.Routes))

	// Validate all route UUIDs
	for i, routeReq := range req.Routes {
		if _, err := uuid.Parse(routeReq.MasterRouteID); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("Invalid master_route_id format for route %d", i+1),
			})
			return
		}
		if _, err := uuid.Parse(routeReq.StopBeforeID); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("Invalid stop_before_id format for route %d", i+1),
			})
			return
		}
		if _, err := uuid.Parse(routeReq.StopAfterID); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("Invalid stop_after_id format for route %d", i+1),
			})
			return
		}
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge owner for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounge owner",
		})
		return
	}

	if owner == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge owner record not found",
		})
		return
	}

	log.Printf("INFO: Current registration step for user %s: %s", userCtx.UserID, owner.RegistrationStep)

	// Check if previous steps are completed (must have completed business info)
	// New flow: phone_verified -> business_info -> lounge_added -> completed
	if owner.RegistrationStep != models.RegStepBusinessInfo && owner.RegistrationStep != models.RegStepLoungeAdded && owner.RegistrationStep != models.RegStepCompleted {
		log.Printf("ERROR: User %s attempted to add lounge with invalid step: %s", userCtx.UserID, owner.RegistrationStep)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "incomplete_registration",
			Message: "Please complete business information step first",
		})
		return
	}

	// Convert amenities and images to JSON strings for JSONB columns
	amenitiesJSON, _ := json.Marshal(req.Amenities)
	imagesJSON, _ := json.Marshal(req.Images)

	// Create lounge (without route info)
	lounge, err := h.loungeRepo.CreateLounge(
		owner.ID,
		req.LoungeName,
		req.Address,
		req.ContactPhone,
		req.Latitude,
		req.Longitude,
		req.Capacity,
		req.Price1Hour,
		req.Price2Hours,
		req.Price3Hours,
		req.PriceUntilBus,
		string(amenitiesJSON),
		string(imagesJSON),
	)
	if err != nil {
		log.Printf("ERROR: Failed to create lounge for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "creation_failed",
			Message: "Failed to create lounge: " + err.Error(),
		})
		return
	}

	// Create lounge routes
	for i, routeReq := range req.Routes {
		masterRouteID, _ := uuid.Parse(routeReq.MasterRouteID)
		stopBeforeID, _ := uuid.Parse(routeReq.StopBeforeID)
		stopAfterID, _ := uuid.Parse(routeReq.StopAfterID)

		loungeRoute := &models.LoungeRoute{
			ID:            uuid.New(),
			LoungeID:      lounge.ID,
			MasterRouteID: masterRouteID,
			StopBeforeID:  stopBeforeID,
			StopAfterID:   stopAfterID,
		}

		if err := h.loungeRouteRepo.CreateLoungeRoute(loungeRoute); err != nil {
			log.Printf("ERROR: Failed to create lounge route %d for lounge %s: %v", i+1, lounge.ID, err)
			// Note: Lounge was created, but routes failed - consider transaction
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "creation_failed",
				Message: "Failed to create lounge routes",
			})
			return
		}
	}

	// Mark registration as completed (sets profile_completed = TRUE and registration_step = 'completed')
	err = h.loungeOwnerRepo.CompleteRegistration(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to complete registration for user %s: %v", userCtx.UserID, err)
		// Continue anyway - lounge was created successfully
	} else {
		log.Printf("INFO: Registration completed for user %s", userCtx.UserID)
	}

	log.Printf("INFO: Lounge created successfully for lounge owner %s (lounge_id: %s)", userCtx.UserID, lounge.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":           "Lounge added successfully",
		"lounge_id":         lounge.ID,
		"registration_step": models.RegStepCompleted, // ✅ Now 'completed' instead of 'lounge_added'
		"profile_completed": true,                    // ✅ Explicitly return this
		"status":            lounge.Status,
	})
}

// ===================================================================
// GET MY LOUNGES
// ===================================================================

// GetMyLounges handles GET /api/v1/lounges/my-lounges
func (h *LoungeHandler) GetMyLounges(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge owner for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	if owner == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge owner not found",
		})
		return
	}

	// Get lounges
	lounges, err := h.loungeRepo.GetLoungesByOwnerID(owner.ID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges for owner %s: %v", owner.ID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	// Convert to response format
	response := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		// Parse JSONB fields
		var amenities []string
		var images []string

		if lounge.Amenities != nil {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if lounge.Images != nil {
			json.Unmarshal(lounge.Images, &images)
		}

		// Get routes for this lounge
		loungeRoutes, err := h.loungeRouteRepo.GetLoungeRoutes(lounge.ID)
		if err != nil {
			log.Printf("WARNING: Failed to get routes for lounge %s: %v", lounge.ID, err)
			loungeRoutes = []models.LoungeRoute{} // Empty array on error
		}

		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_name":     lounge.LoungeName,
			"address":         lounge.Address,
			"contact_phone":   lounge.ContactPhone,
			"latitude":        lounge.Latitude,
			"longitude":       lounge.Longitude,
			"capacity":        lounge.Capacity,
			"price_1_hour":    lounge.Price1Hour,
			"price_2_hours":   lounge.Price2Hours,
			"price_3_hours":   lounge.Price3Hours,
			"price_until_bus": lounge.PriceUntilBus,
			"amenities":       amenities,
			"images":          images,
			"routes":          loungeRoutes,
			"status":          lounge.Status,
			"is_operational":  lounge.IsOperational,
			"average_rating":  lounge.AverageRating,
			"created_at":      lounge.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges": response,
		"total":   len(response),
	})
}

// ===================================================================
// GET LOUNGE BY ID
// ===================================================================

// GetLoungeByID handles GET /api/v1/lounges/:id
func (h *LoungeHandler) GetLoungeByID(c *gin.Context) {
	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounge",
		})
		return
	}

	if lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	// Parse JSONB fields
	var amenities []string
	var images []string

	if lounge.Amenities != nil {
		json.Unmarshal(lounge.Amenities, &amenities)
	}
	if lounge.Images != nil {
		json.Unmarshal(lounge.Images, &images)
	}

	// Get routes for this lounge
	loungeRoutes, err := h.loungeRouteRepo.GetLoungeRoutes(lounge.ID)
	if err != nil {
		log.Printf("WARNING: Failed to get routes for lounge %s: %v", lounge.ID, err)
		loungeRoutes = []models.LoungeRoute{} // Empty array on error
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              lounge.ID,
		"lounge_owner_id": lounge.LoungeOwnerID,
		"lounge_name":     lounge.LoungeName,
		"address":         lounge.Address,
		"contact_phone":   lounge.ContactPhone,
		"latitude":        lounge.Latitude,
		"longitude":       lounge.Longitude,
		"capacity":        lounge.Capacity,
		"price_1_hour":    lounge.Price1Hour,
		"price_2_hours":   lounge.Price2Hours,
		"price_3_hours":   lounge.Price3Hours,
		"price_until_bus": lounge.PriceUntilBus,
		"amenities":       amenities,
		"images":          images,
		"routes":          loungeRoutes,
		"status":          lounge.Status,
		"is_operational":  lounge.IsOperational,
		"average_rating":  lounge.AverageRating,
		"created_at":      lounge.CreatedAt,
		"updated_at":      lounge.UpdatedAt,
	})
}

// ===================================================================
// UPDATE LOUNGE
// ===================================================================

// UpdateLoungeRequest represents the lounge update request
type UpdateLoungeRequest struct {
	LoungeName    string   `json:"lounge_name" binding:"required"`
	Address       string   `json:"address" binding:"required"`
	ContactPhone  string   `json:"contact_phone" binding:"required"`
	Latitude      *string  `json:"latitude" binding:"required"`
	Longitude     *string  `json:"longitude" binding:"required"`
	Capacity      *int     `json:"capacity"`
	Price1Hour    *string  `json:"price_1_hour"`
	Price2Hours   *string  `json:"price_2_hours"`
	Price3Hours   *string  `json:"price_3_hours"`
	PriceUntilBus *string  `json:"price_until_bus"`
	Amenities     []string `json:"amenities"`
	Images        []string `json:"images"`
	// Routes that the lounge serves (array of route-stop combinations)
	Routes []models.LoungeRouteRequest `json:"routes" binding:"required,min=1"`
}

// UpdateLounge handles PUT /api/v1/lounges/:id
func (h *LoungeHandler) UpdateLounge(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	var req UpdateLoungeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify ownership
	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to update this lounge",
		})
		return
	}

	// Convert amenities and images to JSON strings for JSONB columns
	amenitiesJSON, _ := json.Marshal(req.Amenities)
	imagesJSON, _ := json.Marshal(req.Images)

	// Validate all route UUIDs
	for i, routeReq := range req.Routes {
		if _, err := uuid.Parse(routeReq.MasterRouteID); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("Invalid master_route_id format for route %d", i+1),
			})
			return
		}
		if _, err := uuid.Parse(routeReq.StopBeforeID); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("Invalid stop_before_id format for route %d", i+1),
			})
			return
		}
		if _, err := uuid.Parse(routeReq.StopAfterID); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("Invalid stop_after_id format for route %d", i+1),
			})
			return
		}
	}

	// Update lounge (basic info)
	err = h.loungeRepo.UpdateLounge(
		loungeID,
		req.LoungeName,
		req.Address,
		req.ContactPhone,
		req.Latitude,
		req.Longitude,
		req.Capacity,
		req.Price1Hour,
		req.Price2Hours,
		req.Price3Hours,
		req.PriceUntilBus,
		string(amenitiesJSON),
		string(imagesJSON),
	)
	if err != nil {
		log.Printf("ERROR: Failed to update lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update lounge",
		})
		return
	}

	// Delete all existing routes for this lounge
	if err := h.loungeRouteRepo.DeleteAllLoungeRoutes(loungeID); err != nil {
		log.Printf("ERROR: Failed to delete existing routes for lounge %s: %v", loungeID, err)
	}

	// Create new routes
	for i, routeReq := range req.Routes {
		masterRouteID, _ := uuid.Parse(routeReq.MasterRouteID)
		stopBeforeID, _ := uuid.Parse(routeReq.StopBeforeID)
		stopAfterID, _ := uuid.Parse(routeReq.StopAfterID)

		loungeRoute := &models.LoungeRoute{
			ID:            uuid.New(),
			LoungeID:      loungeID,
			MasterRouteID: masterRouteID,
			StopBeforeID:  stopBeforeID,
			StopAfterID:   stopAfterID,
		}

		if err := h.loungeRouteRepo.CreateLoungeRoute(loungeRoute); err != nil {
			log.Printf("ERROR: Failed to create lounge route %d for lounge %s: %v", i+1, loungeID, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "update_failed",
				Message: "Failed to update lounge routes",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lounge updated successfully",
	})
}

// ===================================================================
// DELETE LOUNGE
// ===================================================================

// DeleteLounge handles DELETE /api/v1/lounges/:id
func (h *LoungeHandler) DeleteLounge(c *gin.Context) {
	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify ownership
	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to delete this lounge",
		})
		return
	}

	// Delete lounge (triggers will handle counts)
	err = h.loungeRepo.DeleteLounge(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to delete lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete lounge",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lounge deleted successfully",
	})
}

// ===================================================================
// GET ALL ACTIVE LOUNGES (PUBLIC)
// ===================================================================

// GetAllActiveLounges handles GET /api/v1/lounges/active
// Query params: state (string), limit (int), include_all (bool)
// @Summary Get all active lounges
// @Description Retrieves all active lounges with optional state filter and limit.
//             Routes are loaded in a single JOIN query (no N+1 problem).
// @Tags Lounges
// @Produce json
// @Param state       query string false "Filter by state/province"
// @Param limit       query int    false "Maximum number of lounges to return (random order)"
// @Param include_all query bool   false "If true, return all lounges without limit (still single query)"
// @Success 200 {object} map[string]interface{}
// @Router /lounges/active [get]
func (h *LoungeHandler) GetAllActiveLounges(c *gin.Context) {
	// Apply a 15-second context deadline so the handler always returns
	// within the Choreo gateway timeout, even if the DB is slow.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// Parse query params
	state := c.Query("state")
	var limit int
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// ── Step 1: Fetch lounges (one query) ──────────────────────────────────
	var lounges []models.Lounge
	var err error

	if state != "" || limit > 0 {
		lounges, err = h.loungeRepo.SearchActiveLounges(state, limit)
	} else {
		lounges, err = h.loungeRepo.GetAllActiveLounges()
	}

	if err != nil {
		// Check if it was a context timeout
		if ctx.Err() != nil {
			log.Printf("ERROR: GetAllActiveLounges timed out after 15s: %v", err)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{
				Error:   "timeout",
				Message: "Request timed out retrieving lounges",
			})
			return
		}
		log.Printf("ERROR: Failed to get active lounges: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	if len(lounges) == 0 {
		c.JSON(http.StatusOK, gin.H{"lounges": []gin.H{}, "total": 0})
		return
	}

	// ── Step 2: Collect lounge IDs so we can bulk-load routes ─────────────
	loungeIDs := make([]uuid.UUID, len(lounges))
	for i, l := range lounges {
		loungeIDs[i] = l.ID
	}

	// ── Step 3: Bulk-load ALL routes for these lounges in ONE query ────────
	// This replaces the old per-lounge h.loungeRouteRepo.GetLoungeRoutes() call
	// inside the loop (N+1 problem that caused the 26-second timeout).
	type routeRow struct {
		LoungeID      uuid.UUID `db:"lounge_id"`
		ID            uuid.UUID `db:"id"`
		MasterRouteID uuid.UUID `db:"master_route_id"`
		StopBeforeID  uuid.UUID `db:"stop_before_id"`
		StopAfterID   uuid.UUID `db:"stop_after_id"`
	}

	// Build parameterised IN clause
	query := `SELECT lounge_id, id, master_route_id, stop_before_id, stop_after_id
	          FROM lounge_routes WHERE lounge_id = ANY($1)`
	var routeRows []routeRow
	if dbErr := h.loungeRouteRepo.DB().SelectContext(ctx, &routeRows, query, loungeIDs); dbErr != nil {
		// Route data is non-critical — fall back to empty routes rather than failing
		log.Printf("WARNING: Failed to bulk-load lounge routes: %v", dbErr)
		routeRows = nil
	}

	// Index routes by lounge ID
	routesByLounge := make(map[uuid.UUID][]models.LoungeRoute, len(lounges))
	for _, rr := range routeRows {
		routesByLounge[rr.LoungeID] = append(routesByLounge[rr.LoungeID], models.LoungeRoute{
			ID:            rr.ID,
			LoungeID:      rr.LoungeID,
			MasterRouteID: rr.MasterRouteID,
			StopBeforeID:  rr.StopBeforeID,
			StopAfterID:   rr.StopAfterID,
		})
	}

	// ── Step 4: Build response ─────────────────────────────────────────────
	response := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		var amenities []string
		var images []string
		if lounge.Amenities != nil {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if lounge.Images != nil {
			json.Unmarshal(lounge.Images, &images)
		}

		loungeRoutes := routesByLounge[lounge.ID]
		if loungeRoutes == nil {
			loungeRoutes = []models.LoungeRoute{}
		}

		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_name":     lounge.LoungeName,
			"address":         lounge.Address,
			"latitude":        lounge.Latitude,
			"longitude":       lounge.Longitude,
			"capacity":        lounge.Capacity,
			"price_1_hour":    lounge.Price1Hour,
			"price_2_hours":   lounge.Price2Hours,
			"price_3_hours":   lounge.Price3Hours,
			"price_until_bus": lounge.PriceUntilBus,
			"amenities":       amenities,
			"images":          images,
			"routes":          loungeRoutes,
			"average_rating":  lounge.AverageRating,
			"state":           lounge.State.String,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges": response,
		"total":   len(response),
	})
}


// GetDistinctStates handles GET /api/v1/lounges/states
// @Summary Get all distinct states with active lounges
// @Description Returns a list of states/provinces that have active lounges
// @Tags Lounges
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /lounges/states [get]
func (h *LoungeHandler) GetDistinctStates(c *gin.Context) {
	states, err := h.loungeRepo.GetDistinctStates()
	if err != nil {
		log.Printf("ERROR: Failed to get distinct states: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve states",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"states": states,
		"total":  len(states),
	})
}

// GetLoungesByStop handles GET /api/v1/lounges/by-stop/:stopId
// @Summary Get lounges that serve a specific stop
// @Description Returns all active lounges that serve the given bus stop (as either stop_before or stop_after)
// @Tags Lounges
// @Produce json
// @Param stopId path string true "Stop ID (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /lounges/by-stop/{stopId} [get]
func (h *LoungeHandler) GetLoungesByStop(c *gin.Context) {
	stopIDStr := c.Param("stopId")
	stopID, err := uuid.Parse(stopIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid stop ID format",
		})
		return
	}

	lounges, err := h.loungeRepo.GetLoungesByStopID(stopID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges by stop %s: %v", stopID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	// Build response with parsed JSON fields
	response := make([]gin.H, 0)
	for _, lounge := range lounges {
		// Parse amenities JSON
		var amenities []string
		if len(lounge.Amenities) > 0 {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if amenities == nil {
			amenities = []string{}
		}

		// Parse images JSON
		var images []string
		if len(lounge.Images) > 0 {
			json.Unmarshal(lounge.Images, &images)
		}
		if images == nil {
			images = []string{}
		}

		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_owner_id": lounge.LoungeOwnerID,
			"lounge_name":     lounge.LoungeName,
			"description":     lounge.Description.String,
			"address":         lounge.Address,
			"contact_phone":   lounge.ContactPhone.String,
			"latitude":        lounge.Latitude.String,
			"longitude":       lounge.Longitude.String,
			"capacity":        lounge.Capacity.Int64,
			"price_1_hour":    lounge.Price1Hour.String,
			"price_2_hours":   lounge.Price2Hours.String,
			"price_3_hours":   lounge.Price3Hours.String,
			"price_until_bus": lounge.PriceUntilBus.String,
			"status":          lounge.Status,
			"is_operational":  lounge.IsOperational,
			"amenities":       amenities,
			"images":          images,
			"average_rating":  lounge.AverageRating.String,
			"state":           lounge.State.String,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges": response,
		"stop_id": stopID,
		"total":   len(response),
	})
}

// GetLoungesByMultipleRoutes handles GET /api/v1/lounges/by-multiple-routes?routeIds=id1,id2,...
// @Summary Get lounges that serve ALL of the given routes (intersection)
// @Description Returns active lounges that have a lounge_routes record for every routeId provided.
// @Description Used for transit journeys to find lounges valid on both Leg1 and Leg2 routes.
// @Tags Lounges
// @Produce json
// @Param routeIds query string true "Comma-separated list of master_route_id UUIDs"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /lounges/by-multiple-routes [get]
func (h *LoungeHandler) GetLoungesByMultipleRoutes(c *gin.Context) {
	routeIdsParam := c.Query("routeIds")
	if routeIdsParam == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_param",
			Message: "routeIds query parameter is required",
		})
		return
	}

	// Split and parse each UUID
	parts := splitAndTrim(routeIdsParam, ",")
	routeIDs := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		id, err := uuid.Parse(part)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_id",
				Message: fmt.Sprintf("Invalid route ID format: %s", part),
			})
			return
		}
		routeIDs = append(routeIDs, id)
	}

	lounges, err := h.loungeRepo.GetLoungesByMultipleRouteIDs(routeIDs)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges by multiple routes %v: %v", routeIDs, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	// Build response
	response := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		var amenities []string
		if len(lounge.Amenities) > 0 {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if amenities == nil {
			amenities = []string{}
		}
		var images []string
		if len(lounge.Images) > 0 {
			json.Unmarshal(lounge.Images, &images)
		}
		if images == nil {
			images = []string{}
		}
		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_owner_id": lounge.LoungeOwnerID,
			"lounge_name":     lounge.LoungeName,
			"description":     lounge.Description.String,
			"address":         lounge.Address,
			"contact_phone":   lounge.ContactPhone.String,
			"latitude":        lounge.Latitude.String,
			"longitude":       lounge.Longitude.String,
			"capacity":        lounge.Capacity.Int64,
			"price_1_hour":    lounge.Price1Hour.String,
			"price_2_hours":   lounge.Price2Hours.String,
			"price_3_hours":   lounge.Price3Hours.String,
			"price_until_bus": lounge.PriceUntilBus.String,
			"status":          lounge.Status,
			"is_operational":  lounge.IsOperational,
			"amenities":       amenities,
			"images":          images,
			"average_rating":  lounge.AverageRating.String,
			"state":           lounge.State.String,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges":   response,
		"route_ids": routeIDs,
		"total":     len(response),
	})
}

// splitAndTrim splits a string by sep and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	raw := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || string(s[i]) == sep {
			part := s[start:i]
			// trim spaces
			trimmed := ""
			for j := 0; j < len(part); j++ {
				if part[j] != ' ' && part[j] != '\t' {
					trimmed = part[j:]
					break
				}
			}
			end := len(trimmed)
			for end > 0 && (trimmed[end-1] == ' ' || trimmed[end-1] == '\t') {
				end--
			}
			if end > 0 {
				raw = append(raw, trimmed[:end])
			}
			start = i + 1
		}
	}
	return raw
}

// GetLoungesByRoute handles GET /api/v1/lounges/by-route/:routeId
// @Summary Get lounges that serve a specific route
// @Description Returns all active lounges that serve the given route
// @Tags Lounges
// @Produce json
// @Param routeId path string true "Route ID (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /lounges/by-route/{routeId} [get]
func (h *LoungeHandler) GetLoungesByRoute(c *gin.Context) {
	routeIDStr := c.Param("routeId")
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid route ID format",
		})
		return
	}

	lounges, err := h.loungeRepo.GetLoungesByRouteID(routeID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges by route %s: %v", routeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	// Build response with parsed JSON fields
	response := make([]gin.H, 0)
	for _, lounge := range lounges {
		// Parse amenities JSON
		var amenities []string
		if len(lounge.Amenities) > 0 {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if amenities == nil {
			amenities = []string{}
		}

		// Parse images JSON
		var images []string
		if len(lounge.Images) > 0 {
			json.Unmarshal(lounge.Images, &images)
		}
		if images == nil {
			images = []string{}
		}

		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_owner_id": lounge.LoungeOwnerID,
			"lounge_name":     lounge.LoungeName,
			"description":     lounge.Description.String,
			"address":         lounge.Address,
			"contact_phone":   lounge.ContactPhone.String,
			"latitude":        lounge.Latitude.String,
			"longitude":       lounge.Longitude.String,
			"capacity":        lounge.Capacity.Int64,
			"price_1_hour":    lounge.Price1Hour.String,
			"price_2_hours":   lounge.Price2Hours.String,
			"price_3_hours":   lounge.Price3Hours.String,
			"price_until_bus": lounge.PriceUntilBus.String,
			"status":          lounge.Status,
			"is_operational":  lounge.IsOperational,
			"amenities":       amenities,
			"images":          images,
			"average_rating":  lounge.AverageRating.String,
			"state":           lounge.State.String,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges":  response,
		"route_id": routeID,
		"total":    len(response),
	})
}

// GetLoungesByRouteNear handles GET /api/v1/lounges/by-route-near
// @Summary Get lounges that serve a specific route and are within a GPS radius
// @Description Returns active lounges registered on the route (via lounge_routes) AND within radiusMeters of the given lat/lng. Ordered nearest-first.
// @Tags Lounges
// @Produce json
// @Param routeId query string true  "Master Route ID (UUID)"
// @Param lat     query number true  "Latitude of search point"
// @Param lng     query number true  "Longitude of search point"
// @Param radius  query number false "Search radius in metres (default 20000)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /lounges/by-route-near [get]
func (h *LoungeHandler) GetLoungesByRouteNear(c *gin.Context) {
	routeIDStr := c.Query("routeId")
	if routeIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_param",
			Message: "routeId query parameter is required",
		})
		return
	}
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_param",
			Message: "routeId must be a valid UUID",
		})
		return
	}

	var lat, lng float64
	if _, err := fmt.Sscanf(c.Query("lat"), "%f", &lat); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_param",
			Message: "lat must be a valid float",
		})
		return
	}
	if _, err := fmt.Sscanf(c.Query("lng"), "%f", &lng); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_param",
			Message: "lng must be a valid float",
		})
		return
	}

	radius := 20000.0 // default 20 km
	if r := c.Query("radius"); r != "" {
		fmt.Sscanf(r, "%f", &radius)
	}

	log.Printf("GetLoungesByRouteNear: route=%s lat=%.4f lng=%.4f radius=%.0fm", routeID, lat, lng, radius)

	lounges, err := h.loungeRepo.GetLoungesByRouteNearLocation(routeID, lat, lng, radius)
	if err != nil {
		log.Printf("ERROR: GetLoungesByRouteNear: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	response := make([]gin.H, 0, len(lounges))
	for _, lounge := range lounges {
		var amenities []string
		if len(lounge.Amenities) > 0 {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if amenities == nil {
			amenities = []string{}
		}
		var images []string
		if len(lounge.Images) > 0 {
			json.Unmarshal(lounge.Images, &images)
		}
		if images == nil {
			images = []string{}
		}
		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_owner_id": lounge.LoungeOwnerID,
			"lounge_name":     lounge.LoungeName,
			"description":     lounge.Description.String,
			"address":         lounge.Address,
			"contact_phone":   lounge.ContactPhone.String,
			"latitude":        lounge.Latitude.String,
			"longitude":       lounge.Longitude.String,
			"capacity":        lounge.Capacity.Int64,
			"price_1_hour":    lounge.Price1Hour.String,
			"price_2_hours":   lounge.Price2Hours.String,
			"price_3_hours":   lounge.Price3Hours.String,
			"price_until_bus": lounge.PriceUntilBus.String,
			"status":          lounge.Status,
			"is_operational":  lounge.IsOperational,
			"amenities":       amenities,
			"images":          images,
			"average_rating":  lounge.AverageRating.String,
			"state":           lounge.State.String,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges":   response,
		"route_id":  routeID,
		"latitude":  lat,
		"longitude": lng,
		"radius_m":  radius,
		"total":     len(response),
	})
}


// GetLoungesNearStop handles GET /api/v1/lounges/near-stop/:routeId/:stopId
// @Summary Get lounges near a passenger's selected stop
// @Description Returns all active lounges where the passenger's stop is within 2 stops of the lounge's location
// @Tags Lounges
// @Produce json
// @Param routeId path string true "Master Route ID (UUID)"
// @Param stopId path string true "Passenger's selected stop ID (UUID)"
// @Param distance query int false "Max stop distance (default 2)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /lounges/near-stop/{routeId}/{stopId} [get]
func (h *LoungeHandler) GetLoungesNearStop(c *gin.Context) {
	routeIDStr := c.Param("routeId")
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid route ID format",
		})
		return
	}

	stopIDStr := c.Param("stopId")
	stopID, err := uuid.Parse(stopIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid stop ID format",
		})
		return
	}

	// Default max distance is 2 stops
	maxDistance := 2
	if distStr := c.Query("distance"); distStr != "" {
		if dist, err := strconv.Atoi(distStr); err == nil && dist > 0 {
			maxDistance = dist
		}
	}

	log.Printf("INFO: Finding lounges near stop %s on route %s (max distance: %d stops)", stopID, routeID, maxDistance)

	lounges, err := h.loungeRepo.GetLoungesNearStop(routeID, stopID, maxDistance)
	if err != nil {
		log.Printf("ERROR: Failed to get lounges near stop %s on route %s: %v", stopID, routeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounges",
		})
		return
	}

	log.Printf("INFO: Found %d lounges near stop %s", len(lounges), stopID)

	// Build response with parsed JSON fields
	response := make([]gin.H, 0)
	for _, lounge := range lounges {
		// Parse amenities JSON
		var amenities []string
		if len(lounge.Amenities) > 0 {
			json.Unmarshal(lounge.Amenities, &amenities)
		}
		if amenities == nil {
			amenities = []string{}
		}

		// Parse images JSON
		var images []string
		if len(lounge.Images) > 0 {
			json.Unmarshal(lounge.Images, &images)
		}
		if images == nil {
			images = []string{}
		}

		response = append(response, gin.H{
			"id":              lounge.ID,
			"lounge_owner_id": lounge.LoungeOwnerID,
			"lounge_name":     lounge.LoungeName,
			"description":     lounge.Description.String,
			"address":         lounge.Address,
			"contact_phone":   lounge.ContactPhone.String,
			"latitude":        lounge.Latitude.String,
			"longitude":       lounge.Longitude.String,
			"capacity":        lounge.Capacity.Int64,
			"price_1_hour":    lounge.Price1Hour.String,
			"price_2_hours":   lounge.Price2Hours.String,
			"price_3_hours":   lounge.Price3Hours.String,
			"price_until_bus": lounge.PriceUntilBus.String,
			"status":          lounge.Status,
			"is_operational":  lounge.IsOperational,
			"amenities":       amenities,
			"images":          images,
			"average_rating":  lounge.AverageRating.String,
			"state":           lounge.State.String,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lounges":      response,
		"route_id":     routeID,
		"stop_id":      stopID,
		"max_distance": maxDistance,
		"total":        len(response),
	})
}
