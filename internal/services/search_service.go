package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// SearchService handles business logic for trip search
type SearchService struct {
	repo   *database.SearchRepository
	logger *logrus.Logger
}

// NewSearchService creates a new search service
func NewSearchService(repo *database.SearchRepository, logger *logrus.Logger) *SearchService {
	return &SearchService{
		repo:   repo,
		logger: logger,
	}
}

// radiusSteps defines the incremental search radius expansion in meters.
// The algorithm tries each radius in sequence (3km steps) until lounges are found.
var radiusSteps = []float64{3000, 6000, 9000, 12000, 15000}

// SearchTrips implements the Lounge-centric Direct Route Discovery algorithm.
// It uses a single-query approach with incremental radius expansion to find
// direct bus paths between the nearest candidate lounges.
func (s *SearchService) SearchTrips(
	req *models.SearchRequest,
	userID *uuid.UUID,
	ipAddress string,
) (*models.SearchResponse, error) {
	startTime := time.Now()

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Coordinates are REQUIRED for lounge-centric search.
	// If missing, attempt to resolve them from names.
	fromLat, fromLng := req.FromLat, req.FromLng
	toLat, toLng := req.ToLat, req.ToLng

	if fromLat == nil || fromLng == nil {
		s.logger.WithField("location", req.From).Info("Resolving origin coordinates...")
		lat, lng, err := s.repo.ResolveCoordinates(req.From)
		if err == nil {
			fromLat = &lat
			fromLng = &lng
		}
	}

	if toLat == nil || toLng == nil {
		s.logger.WithField("location", req.To).Info("Resolving destination coordinates...")
		lat, lng, err := s.repo.ResolveCoordinates(req.To)
		if err == nil {
			toLat = &lat
			toLng = &lng
		}
	}

	s.logger.WithFields(logrus.Fields{
		"from":     req.From,
		"to":       req.To,
		"from_lat": fromLat,
		"from_lng": fromLng,
		"to_lat":   toLat,
		"to_lng":   toLng,
	}).Info("=== SearchService: Starting Lounge-centric Direct Route Discovery ===")

	searchTime := req.GetSearchDateTime()

	// --- SMART MULTI-MODAL RADIUS SEARCH ---
	// Strategy:
	//   1. Expand radius until ANY routes are found (Direct or Transit).
	//   2. Once a radius succeeds, lock it — the SQL already returns BOTH types
	//      in one query (direct ordered first, transit second).
	//   3. Smart 3 km cap: if results come back at 3km we stop expanding.
	//      The user sees both Direct and Transit options simultaneously.
	var results []models.TripResult
	var usedRadius float64

	if fromLat != nil && fromLng != nil && toLat != nil && toLng != nil {
		for _, radius := range radiusSteps {
			if req.RadiusLimit != nil && radius > *req.RadiusLimit {
				break
			}
			usedRadius = radius
			s.logger.WithField("radius_m", radius).Info("Running multi-modal lounge search...")

			var err error
			results, err = s.repo.FindMultiModalRoutes(
				*fromLat, *fromLng,
				*toLat, *toLng,
				radius,
				searchTime,
				req.Limit,
			)
			if err != nil {
				s.logger.WithError(err).WithField("radius_m", radius).Warn("FindMultiModalRoutes error; expanding radius")
				continue
			}

			if len(results) > 0 {
				directCount, transitCount := 0, 0
				for _, r := range results {
					if r.IsTransit {
						transitCount++
					} else {
						directCount++
					}
				}
				s.logger.WithFields(logrus.Fields{
					"radius_m": radius,
					"direct":   directCount,
					"transit":  transitCount,
				}).Info("Found routes — stopping radius expansion.")
				break
			}
			s.logger.WithField("radius_m", radius).Info("No routes at this radius; expanding...")
		}
	} else {
		s.logger.Warn("Coordinates unavailable; skipping lounge-centric search.")
	}

	// Determine search type — mixed results are now possible
	searchType := "lounge_direct"
	hasTransit, hasDirect := false, false
	for _, r := range results {
		if r.IsTransit {
			hasTransit = true
		} else {
			hasDirect = true
		}
	}
	if hasDirect && hasTransit {
		searchType = "lounge_multimodal"
	} else if hasTransit {
		searchType = "lounge_transit"
	}

	// --- FALLBACK TO STATIC ROUTE SEARCH IF NO SCHEDULED TRIPS FOUND ---
	staticRouteMessage := ""
	if len(results) == 0 && fromLat != nil && fromLng != nil && toLat != nil && toLng != nil {
		s.logger.Info("No scheduled trips found, attempting static route discovery")
		staticResults, err := s.repo.FindStaticLoungeRoutes(
			*fromLat, *fromLng,
			*toLat, *toLng,
			usedRadius, // Use the same radius that was used for scheduled search
		)
		if err == nil && len(staticResults) > 0 {
			results = staticResults
			searchType = "static_route"
			staticRouteMessage = fmt.Sprintf(
				"A physical route exists via %s, but no buses are scheduled for this date.",
				staticResults[0].RouteName,
			)
			s.logger.WithFields(logrus.Fields{
				"static_routes": len(staticResults),
				"route_name":    staticResults[0].RouteName,
			}).Info("Found static routes — returning ghost trips")
		}
	}

	// --- FALLBACK TO REGULAR SEARCH IF NO LOUNGES FOUND ---
	fallbackMessage := ""
	if len(results) == 0 {
		s.logger.Info("No lounges found, falling back to regular stop-to-stop search")
		pair, err := s.repo.FindStopPairOnSameRoute(req.From, req.To)
		if err == nil && pair.Matched {
			results, err = s.repo.FindDirectTrips(pair.FromID, pair.ToID, searchTime, req.Limit)
			if err == nil && len(results) > 0 {
				searchType = "stop_direct"
				fallbackMessage = fmt.Sprintf("No lounges found near your locations. Found %d regular schedules from '%s' to '%s' instead.", len(results), pair.FromStop.Name, pair.ToStop.Name)
			}
		}
	}

	// Build the response
	response := &models.SearchResponse{
		Status: "success",
		SearchDetails: models.SearchDetails{
			FromStop: models.StopInfo{
				OriginalInput: req.From,
				Matched:       len(results) > 0,
			},
			ToStop: models.StopInfo{
				OriginalInput: req.To,
				Matched:       len(results) > 0,
			},
			SearchType: searchType,
		},
		Results:    results,
		RadiusUsed: usedRadius,
	}

	if len(results) == 0 {
		response.Status = "success"
		response.Message = fmt.Sprintf(
			"No routes found from '%s' to '%s' even after expanding search radius to %.0fkm. Please try a different date or time.",
			req.From, req.To, usedRadius/1000,
		)
		response.RouteExists = false
		response.DiscoveryStatus = "no_routes"
	} else if staticRouteMessage != "" {
		response.Message = staticRouteMessage
		response.RouteExists = true
		response.DiscoveryStatus = "route_available"
	} else if fallbackMessage != "" {
		response.Message = fallbackMessage
		response.RouteExists = true
		response.DiscoveryStatus = "scheduled_available"
	} else {
		response.Message = fmt.Sprintf(
			"Found %d direct lounge route(s) from '%s' to '%s' (search radius: %.0fkm).",
			len(results), req.From, req.To, usedRadius/1000,
		)
		response.RouteExists = true
		response.DiscoveryStatus = "scheduled_available"
	}

	// Log timing
	responseTime := time.Since(startTime)
	response.SearchTimeMs = responseTime.Milliseconds()

	// Async analytics logging
	s.logSearch(req, response, userID, &ipAddress, responseTime)

	s.logger.WithFields(logrus.Fields{
		"results":     len(results),
		"radius_used": usedRadius,
		"response_ms": response.SearchTimeMs,
	}).Info("=== SearchService: Lounge-centric search completed ===")

	return response, nil
}

// GetPopularRoutes returns popular routes for quick selection
func (s *SearchService) GetPopularRoutes(limit int) ([]models.PopularRoute, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	routes, err := s.repo.GetPopularRoutes(limit)
	if err != nil {
		s.logger.WithError(err).Error("Error getting popular routes")
		return nil, fmt.Errorf("error retrieving popular routes: %w", err)
	}

	if len(routes) == 0 {
		routes = s.getDefaultPopularRoutes()
	}

	return routes, nil
}

// GetStopAutocomplete returns stop suggestions for autocomplete
func (s *SearchService) GetStopAutocomplete(searchTerm string, limit int) ([]models.StopAutocomplete, error) {
	if searchTerm == "" || len(searchTerm) < 2 {
		return []models.StopAutocomplete{}, nil
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	suggestions, err := s.repo.GetStopAutocomplete(searchTerm, limit)
	if err != nil {
		s.logger.WithError(err).Error("Error getting autocomplete suggestions")
		return nil, fmt.Errorf("error retrieving suggestions: %w", err)
	}

	return suggestions, nil
}

// GetSearchAnalytics returns search analytics for admin dashboard
func (s *SearchService) GetSearchAnalytics(days int) (map[string]interface{}, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}

	analytics, err := s.repo.GetSearchAnalytics(days)
	if err != nil {
		s.logger.WithError(err).Error("Error getting search analytics")
		return nil, fmt.Errorf("error retrieving analytics: %w", err)
	}

	return analytics, nil
}

// logSearch logs the search request for analytics asynchronously.
func (s *SearchService) logSearch(
	req *models.SearchRequest,
	response *models.SearchResponse,
	userID *uuid.UUID,
	ipAddress *string,
	responseTime time.Duration,
) {
	log := &models.SearchLog{
		FromInput:      req.From,
		ToInput:        req.To,
		ResultsCount:   len(response.Results),
		ResponseTimeMs: responseTime.Milliseconds(),
		UserID:         userID,
		IPAddress:      ipAddress,
	}

	go func() {
		if err := s.repo.LogSearch(log); err != nil {
			s.logger.WithError(err).Warn("Failed to log search")
		}
	}()
}

// getDefaultPopularRoutes returns hardcoded popular routes for Sri Lanka
func (s *SearchService) getDefaultPopularRoutes() []models.PopularRoute {
	return []models.PopularRoute{
		{FromStopName: "Colombo Fort", ToStopName: "Kandy", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Galle", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Anuradhapura", RouteCount: 0},
		{FromStopName: "Kandy", ToStopName: "Nuwara Eliya", RouteCount: 0},
		{FromStopName: "Galle", ToStopName: "Matara", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Jaffna", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Trincomalee", RouteCount: 0},
		{FromStopName: "Kandy", ToStopName: "Ella", RouteCount: 0},
		{FromStopName: "Negombo", ToStopName: "Colombo Fort", RouteCount: 0},
		{FromStopName: "Colombo Fort", ToStopName: "Katunayake", RouteCount: 0},
	}
}
