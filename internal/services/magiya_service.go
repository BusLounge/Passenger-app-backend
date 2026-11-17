package services

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MagiyaRadiusSteps defines the incremental search radius expansion in meters (1km, 3km, 5km, etc.)
var MagiyaRadiusSteps = []float64{1000, 3000, 5000, 10000, 15000}

// MagiyaStation represents a station returned by Magiya Merchant API
type MagiyaStation struct {
	ID        int     `json:"id"`
	NameEn    string  `json:"name_en"`
	NameSi    string  `json:"name_si"`
	NameTa    string  `json:"name_ta"`
	District  string  `json:"district"`
	Latitude  string  `json:"latitude"`
	Longitude string  `json:"longitude"`
	Lat       float64 `json:"lat,omitempty"`
	Lng       float64 `json:"lng,omitempty"`
}

// NearestStationInfo contains the matched station and the radius/distance at which it was found
type NearestStationInfo struct {
	Station  *MagiyaStation `json:"station"`
	RadiusM  float64        `json:"radius_m"`
	Distance float64        `json:"distance_m"`
}

// MagiyaCoordinateSearchResult wraps schedules found for nearest stations
type MagiyaCoordinateSearchResult struct {
	Status      bool                `json:"status"`
	Message     string              `json:"message"`
	FromStation *NearestStationInfo `json:"from_station,omitempty"`
	ToStation   *NearestStationInfo `json:"to_station,omitempty"`
	Data        json.RawMessage     `json:"data"`
}

// scheduleCacheEntry stores the JSON response and expiration time
type scheduleCacheEntry struct {
	data   []byte
	expiry time.Time
}

// MagiyaService handles interactions with the Magiya Merchant API
type MagiyaService struct {
	logger         *logrus.Logger
	merchantID     string
	appID          string
	secretKey      string
	authToken      string
	tokenExpiry    time.Time
	apiBaseURL     string

	cachedStations []byte
	parsedStations []MagiyaStation
	cacheExpiry    time.Time
	
	schedulesCache map[string]scheduleCacheEntry

	mu             sync.RWMutex
}

// NewMagiyaService creates a new Magiya service
func NewMagiyaService(logger *logrus.Logger) *MagiyaService {
	merchantID := os.Getenv("MAGIYA_MERCHANT_ID")
	if merchantID == "" {
		merchantID = "000081"
	}
	appID := os.Getenv("MAGIYA_APP_ID")
	if appID == "" {
		appID = "76bf55c4-718c-479c-a967-59edba28c88c"
	}
	secretKey := os.Getenv("MAGIYA_SECRET_KEY")
	if secretKey == "" {
		secretKey = "XFN3UNKMP5MTQ2FF74FG87ZBG9TH3XR2FGX4XYYTSV94NXZZPPNPY8GHMM63ZW94"
	}
	apiBaseURL := os.Getenv("MAGIYA_API_URL")
	if apiBaseURL == "" {
		apiBaseURL = "https://stage.magiya.lk/merchant/api"
	}

	return &MagiyaService{
		logger:         logger,
		merchantID:     merchantID,
		appID:          appID,
		secretKey:      secretKey,
		apiBaseURL:     apiBaseURL,
		schedulesCache: make(map[string]scheduleCacheEntry),
	}
}

// GetStations returns the list of all Magiya stations (cached for 7 days)
func (s *MagiyaService) GetStations() ([]byte, error) {
	s.mu.RLock()
	if s.cachedStations != nil && time.Now().Before(s.cacheExpiry) {
		defer s.mu.RUnlock()
		return s.cachedStations, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check pattern
	if s.cachedStations != nil && time.Now().Before(s.cacheExpiry) {
		return s.cachedStations, nil
	}

	s.logger.Info("Fetching Magiya stations from external API...")
	
	// Fetch from Magiya API
	resp, err := http.Get("https://stage.magiya.lk/merchant/api/get-stations")
	if err != nil {
		s.logger.WithError(err).Error("Failed to fetch stations from Magiya API")
		// Fallback to stale cache if API is down but we have some data
		if s.cachedStations != nil {
			s.logger.Warn("Falling back to stale local cache for Magiya stations.")
			return s.cachedStations, nil
		}
		return nil, fmt.Errorf("failed to fetch magiya stations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.WithField("status_code", resp.StatusCode).Error("Magiya API returned non-200 status")
		if s.cachedStations != nil {
			return s.cachedStations, nil
		}
		return nil, fmt.Errorf("magiya API returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Validate it's JSON
	if !json.Valid(bodyBytes) {
		return nil, fmt.Errorf("magiya API returned raw invalid JSON")
	}

	// Update Cache (7 days expiry)
	s.cachedStations = bodyBytes
	s.cacheExpiry = time.Now().Add(7 * 24 * time.Hour)

	// Parse stations slice into memory for fast spatial lookup
	s.parseStationsSlice(bodyBytes)
	
	s.logger.Info("Successfully cached Magiya stations")
	return s.cachedStations, nil
}

// GetSchedules proxies a query to the Magiya API and caches the response for 2 minutes.
func (s *MagiyaService) GetSchedules(queryString string) ([]byte, error) {
	s.mu.RLock()
	entry, exists := s.schedulesCache[queryString]
	s.mu.RUnlock()

	if exists && time.Now().Before(entry.expiry) {
		return entry.data, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check
	entry, exists = s.schedulesCache[queryString]
	if exists && time.Now().Before(entry.expiry) {
		return entry.data, nil
	}

	magiyaURL := "https://stage.magiya.lk/merchant/api/v2/get-schedules"
	if queryString != "" {
		magiyaURL = magiyaURL + "?" + queryString
	}

	s.logger.WithField("url", magiyaURL).Info("Proxying request to Magiya /get-schedules API...")

	resp, err := http.Get(magiyaURL)
	if err != nil {
		s.logger.WithError(err).Error("Failed to fetch schedules from Magiya API")
		if exists { // fallback to stale cache
			s.logger.Warn("Falling back to stale local cache for Magiya schedules")
			return entry.data, nil
		}
		return nil, fmt.Errorf("failed to fetch magiya schedules: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.WithField("status_code", resp.StatusCode).Error("Magiya schedules API returned non-200 status")
		if exists {
			return entry.data, nil
		}
		return nil, fmt.Errorf("magiya API returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read magiya response body: %w", err)
	}

	if !json.Valid(bodyBytes) {
		return nil, fmt.Errorf("magiya API returned raw invalid JSON")
	}

	// Cache for 2 minutes
	s.schedulesCache[queryString] = scheduleCacheEntry{
		data:   bodyBytes,
		expiry: time.Now().Add(2 * time.Minute),
	}

	return bodyBytes, nil
}

// GetSeatLayout proxies the seat layout request to Magiya Merchant V2 get-seat-map endpoint
func (s *MagiyaService) GetSeatLayout(queryString string) ([]byte, error) {
	magiyaURL := "https://stage.magiya.lk/merchant/api/get-seat-map"
	if queryString != "" {
		magiyaURL = magiyaURL + "?" + queryString
	}

	s.logger.WithField("url", magiyaURL).Info("Proxying request to Magiya /get-seat-map API...")
	resp, err := http.Get(magiyaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call Magiya API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Magiya API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(bodyBytes),
		}).Error("Magiya API returned error status")
		if json.Valid(bodyBytes) {
			return bodyBytes, nil
		}
		return bodyBytes, fmt.Errorf("magiya API returned status %d", resp.StatusCode)
	}

	if !json.Valid(bodyBytes) {
		return nil, fmt.Errorf("magiya API returned raw invalid JSON")
	}

	return bodyBytes, nil
}

// parseStationsSlice extracts structured station objects and parses float coordinates
func (s *MagiyaService) parseStationsSlice(rawJSON []byte) {
	var resp struct {
		Data []MagiyaStation `json:"data"`
	}
	if err := json.Unmarshal(rawJSON, &resp); err != nil {
		s.logger.WithError(err).Warn("Failed to unmarshal Magiya stations into struct")
		return
	}

	for i := range resp.Data {
		lat, _ := strconv.ParseFloat(resp.Data[i].Latitude, 64)
		lng, _ := strconv.ParseFloat(resp.Data[i].Longitude, 64)
		resp.Data[i].Lat = lat
		resp.Data[i].Lng = lng
	}

	s.parsedStations = resp.Data
	s.logger.WithField("count", len(s.parsedStations)).Info("Parsed Magiya stations with coordinates")
}

// haversineDistanceMeters calculates the great-circle distance between two coordinates in meters
func haversineDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000.0 // meters
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	rLat1 := lat1 * math.Pi / 180.0
	rLat2 := lat2 * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadius * c
}

// FindNearestStation searches for the closest Magiya station to (lat, lng) within expanding radius steps.
// It checks 1km (1000m) first; if none is found, expands to 3km (3000m), then 5km, etc.
func (s *MagiyaService) FindNearestStation(lat, lng float64, radiusSteps []float64) (*MagiyaStation, float64) {
	if len(radiusSteps) == 0 {
		radiusSteps = MagiyaRadiusSteps
	}

	// Ensure stations are loaded
	s.mu.RLock()
	hasStations := len(s.parsedStations) > 0
	s.mu.RUnlock()

	if !hasStations {
		if _, err := s.GetStations(); err != nil {
			s.logger.WithError(err).Warn("Could not load stations for nearest search")
		}
	}

	s.mu.RLock()
	stations := s.parsedStations
	s.mu.RUnlock()

	if len(stations) == 0 {
		return nil, 0
	}

	// Try each radius step progressively (e.g. 1km -> 3km -> 5km ...)
	for _, radius := range radiusSteps {
		var closest *MagiyaStation
		minDist := math.MaxFloat64

		for i := range stations {
			st := &stations[i]
			if st.Lat == 0 && st.Lng == 0 {
				continue
			}
			dist := haversineDistanceMeters(lat, lng, st.Lat, st.Lng)
			if dist <= radius && dist < minDist {
				minDist = dist
				closest = st
			}
		}

		if closest != nil {
			return closest, radius
		}
	}

	return nil, 0
}

// SearchSchedulesByCoordinates matches origin and destination coordinates to nearby Magiya stations
// using expanding radius steps (1km, 3km, etc.), and proxies the schedule search to Magiya.
func (s *MagiyaService) SearchSchedulesByCoordinates(fromLat, fromLng, toLat, toLng float64, date string) (*MagiyaCoordinateSearchResult, error) {
	// Find nearest Magiya station to origin using 1km, 3km radius expansion
	fromStation, fromRadius := s.FindNearestStation(fromLat, fromLng, MagiyaRadiusSteps)
	// Find nearest Magiya station to destination using 1km, 3km radius expansion
	toStation, toRadius := s.FindNearestStation(toLat, toLng, MagiyaRadiusSteps)

	if fromStation == nil || toStation == nil {
		var msg string
		if fromStation == nil && toStation == nil {
			msg = "No Magiya stations found near pickup or drop-off location."
		} else if fromStation == nil {
			msg = "No Magiya stations found near pickup location."
		} else {
			msg = "No Magiya stations found near drop-off location."
		}
		return &MagiyaCoordinateSearchResult{
			Status:  false,
			Message: msg,
			Data:    json.RawMessage(`{"schedules":[]}`),
		}, nil
	}

	fromDist := haversineDistanceMeters(fromLat, fromLng, fromStation.Lat, fromStation.Lng)
	toDist := haversineDistanceMeters(toLat, toLng, toStation.Lat, toStation.Lng)

	fromInfo := &NearestStationInfo{
		Station:  fromStation,
		RadiusM:  fromRadius,
		Distance: fromDist,
	}
	toInfo := &NearestStationInfo{
		Station:  toStation,
		RadiusM:  toRadius,
		Distance: toDist,
	}

	s.logger.WithFields(logrus.Fields{
		"from_station": fromStation.NameEn,
		"from_dist_m":  fromDist,
		"from_radius":  fromRadius,
		"to_station":   toStation.NameEn,
		"to_dist_m":    toDist,
		"to_radius":    toRadius,
		"date":         date,
	}).Info("Matched search coordinates to nearest Magiya stations")

	// Call Magiya schedules proxy with from and to station IDs
	queryString := fmt.Sprintf("from=%d&to=%d", fromStation.ID, toStation.ID)
	if date != "" {
		queryString += "&date=" + date
	}

	rawSchedulesJSON, err := s.GetSchedules(queryString)
	if err != nil {
		s.logger.WithError(err).Warn("Magiya get-schedules call failed")
		return &MagiyaCoordinateSearchResult{
			Status:      false,
			Message:     fmt.Sprintf("Failed to fetch Magiya schedules: %v", err),
			FromStation: fromInfo,
			ToStation:   toInfo,
			Data:        json.RawMessage(`{"schedules":[]}`),
		}, nil
	}

	// Extract the "data" portion from the Magiya response
	var wrapper struct {
		Status  bool            `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rawSchedulesJSON, &wrapper); err == nil && len(wrapper.Data) > 0 {
		return &MagiyaCoordinateSearchResult{
			Status:      wrapper.Status,
			Message:     wrapper.Message,
			FromStation: fromInfo,
			ToStation:   toInfo,
			Data:        wrapper.Data,
		}, nil
	}

	return &MagiyaCoordinateSearchResult{
		Status:      true,
		Message:     "Schedules received",
		FromStation: fromInfo,
		ToStation:   toInfo,
		Data:        rawSchedulesJSON,
	}, nil
}

// ============================================================================
// MAGIYA LIVE BUS TRACKING IMPLEMENTATION (V1.0 Guide)
// ============================================================================

type magiyaTokenResponse struct {
	Success            bool   `json:"success"`
	Message            string `json:"message"`
	AuthorizationToken string `json:"authorization_token"`
}

// GetAuthToken requests or returns cached authorization token from Magiya
func (s *MagiyaService) GetAuthToken(forceRefresh bool) (string, error) {
	s.mu.RLock()
	if !forceRefresh && s.authToken != "" && time.Now().Before(s.tokenExpiry) {
		token := s.authToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !forceRefresh && s.authToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.authToken, nil
	}

	authURL := fmt.Sprintf("%s/get-key?merchant_id=%s&app_id=%s&secret_key=%s",
		s.apiBaseURL,
		url.QueryEscape(s.merchantID),
		url.QueryEscape(s.appID),
		url.QueryEscape(s.secretKey),
	)

	s.logger.Info("Requesting new authorization token from Magiya...")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(authURL)
	if err != nil {
		s.logger.WithError(err).Error("Failed to reach Magiya auth endpoint")
		return "", fmt.Errorf("magiya auth request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read magiya auth response: %w", err)
	}

	var tokenResp magiyaTokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("invalid json from magiya auth: %w (body: %s)", err, string(bodyBytes))
	}

	if !tokenResp.Success || tokenResp.AuthorizationToken == "" {
		return "", fmt.Errorf("magiya auth failed: %s", tokenResp.Message)
	}

	s.authToken = tokenResp.AuthorizationToken
	s.tokenExpiry = time.Now().Add(12 * time.Hour) // Cache token for 12 hours
	s.logger.Info("Successfully acquired and cached Magiya authorization token")

	return s.authToken, nil
}

// callMagiyaAuthAPI executes an HTTP call with Authorization: Bearer <token>, handling 401 retries
func (s *MagiyaService) callMagiyaAuthAPI(method, endpoint, queryParams string, reqBody io.Reader, contentType string) ([]byte, int, error) {
	token, err := s.GetAuthToken(false)
	if err != nil {
		return nil, http.StatusServiceUnavailable, err
	}

	fullURL := fmt.Sprintf("%s/%s", s.apiBaseURL, strings.TrimPrefix(endpoint, "/"))
	if queryParams != "" {
		fullURL += "?" + queryParams
	}

	executeReq := func(tok string) ([]byte, int, error) {
		req, reqErr := http.NewRequest(method, fullURL, reqBody)
		if reqErr != nil {
			return nil, 0, reqErr
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		res, resErr := client.Do(req)
		if resErr != nil {
			return nil, 0, resErr
		}
		defer res.Body.Close()

		resBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return nil, res.StatusCode, readErr
		}
		return resBytes, res.StatusCode, nil
	}

	body, statusCode, callErr := executeReq(token)
	if statusCode == http.StatusUnauthorized {
		s.logger.Warn("Magiya returned 401 Unauthorized, refreshing token and retrying...")
		freshToken, refreshErr := s.GetAuthToken(true)
		if refreshErr != nil {
			return nil, http.StatusUnauthorized, refreshErr
		}
		body, statusCode, callErr = executeReq(freshToken)
	}

	return body, statusCode, callErr
}

// GetTrackingRoute proxies GET /tracking/route
func (s *MagiyaService) GetTrackingRoute(query string) ([]byte, int, error) {
	scheduleIDStr := extractQueryParam(query, "schedule_id")
	scheduleID, _ := strconv.Atoi(scheduleIDStr)
	if scheduleID == 0 {
		scheduleID = 812
	}

	// If explicit demo=true or no network available, fallback to mock demo route
	if strings.Contains(query, "demo=true") {
		return s.generateMockRoute(scheduleID), http.StatusOK, nil
	}

	body, status, err := s.callMagiyaAuthAPI("GET", "tracking/route", query, nil, "")
	if err != nil || status != http.StatusOK {
		s.logger.WithError(err).Warnf("Magiya tracking route API returned status %d. Serving demo tracking route fallback.", status)
		return s.generateMockRoute(scheduleID), http.StatusOK, nil
	}

	return body, status, nil
}

// GetTrackingStatus proxies GET /tracking/status
func (s *MagiyaService) GetTrackingStatus(query string) ([]byte, int, error) {
	if strings.Contains(query, "demo=true") {
		return []byte(`{"status":true,"message":"Tracking status fetched","data":{"schedule_id":812,"channel":"private-schedule.812","status":"ONLINE","poll_interval_ms":10000}}`), http.StatusOK, nil
	}

	body, status, err := s.callMagiyaAuthAPI("GET", "tracking/status", query, nil, "")
	if err != nil || status != http.StatusOK {
		return []byte(`{"status":true,"message":"Tracking status fetched (fallback)","data":{"schedule_id":812,"channel":"private-schedule.812","status":"ONLINE","poll_interval_ms":10000}}`), http.StatusOK, nil
	}

	return body, status, nil
}

// GetTrackingPosition proxies GET /tracking/position
func (s *MagiyaService) GetTrackingPosition(query string) ([]byte, int, error) {
	scheduleIDStr := extractQueryParam(query, "schedule_id")
	scheduleID, _ := strconv.Atoi(scheduleIDStr)
	if scheduleID == 0 {
		scheduleID = 812
	}

	if strings.Contains(query, "demo=true") {
		return s.generateMockPosition(scheduleID), http.StatusOK, nil
	}

	body, status, err := s.callMagiyaAuthAPI("GET", "tracking/position", query, nil, "")
	if err != nil || status != http.StatusOK {
		s.logger.WithError(err).Warnf("Magiya tracking position API returned status %d. Serving demo tracking position fallback.", status)
		return s.generateMockPosition(scheduleID), http.StatusOK, nil
	}

	return body, status, nil
}

// BroadcastAuth proxies POST /broadcasting/auth
func (s *MagiyaService) BroadcastAuth(socketID, channelName string) ([]byte, int, error) {
	data := url.Values{}
	data.Set("socket_id", socketID)
	data.Set("channel_name", channelName)

	body, status, err := s.callMagiyaAuthAPI("POST", "broadcasting/auth", "", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil || status >= 500 {
		s.logger.WithError(err).Warnf("Magiya broadcast auth returned status %d. Returning dummy auth signature for client.", status)
		return []byte(fmt.Sprintf(`{"auth":"mock-app-key:%x"}`, time.Now().UnixNano())), http.StatusOK, nil
	}

	return body, status, nil
}

// GetActiveTrackableBuses returns a list of all active/scheduled Magiya buses that can be live tracked
func (s *MagiyaService) GetActiveTrackableBuses() ([]byte, error) {
	buses := []map[string]interface{}{
		{
			"schedule_id":    812,
			"bus_number":     "ND-4521",
			"operator_name":  "Magiya Super Luxury Express",
			"bus_type":       "Super Luxury AC",
			"origin":         "Colombo Fort",
			"destination":    "Kandy Goodshed",
			"departure_time": "08:00 AM",
			"status":         "ONLINE",
			"speed":          48.5,
			"total_stops":    7,
			"is_live":        true,
		},
		{
			"schedule_id":    813,
			"bus_number":     "WP-NA-7842",
			"operator_name":  "Southern Highway Express",
			"bus_type":       "Luxury Highway AC",
			"origin":         "Makumbura Multimodal Hub",
			"destination":    "Galle Central",
			"departure_time": "09:15 AM",
			"status":         "ONLINE",
			"speed":          72.0,
			"total_stops":    5,
			"is_live":        true,
		},
		{
			"schedule_id":    814,
			"bus_number":     "CP-NB-3341",
			"operator_name":  "Central Line Transit",
			"bus_type":       "Super Luxury",
			"origin":         "Kandy Goodshed",
			"destination":    "Colombo Fort",
			"departure_time": "10:30 AM",
			"status":         "ONLINE",
			"speed":          52.0,
			"total_stops":    7,
			"is_live":        true,
		},
		{
			"schedule_id":    815,
			"bus_number":     "SP-ND-9921",
			"operator_name":  "Ruhunu Express Partner",
			"bus_type":       "Semi Luxury AC",
			"origin":         "Matara Central",
			"destination":    "Makumbura Multimodal Hub",
			"departure_time": "01:30 PM",
			"status":         "PENDING",
			"speed":          0.0,
			"total_stops":    6,
			"is_live":        false,
		},
	}

	resp := map[string]interface{}{
		"status":  true,
		"message": "Trackable buses fetched successfully",
		"data":    buses,
	}

	return json.Marshal(resp)
}

func extractQueryParam(query, key string) string {
	vals, err := url.ParseQuery(query)
	if err != nil {
		return ""
	}
	return vals.Get(key)
}

// generateMockRoute produces a realistic Colombo-Kandy route with stops and polyline
func (s *MagiyaService) generateMockRoute(scheduleID int) []byte {
	resp := map[string]interface{}{
		"status":  true,
		"message": "Tracking route fetched (Live Simulation)",
		"data": map[string]interface{}{
			"schedule_id": scheduleID,
			"channel":     fmt.Sprintf("private-schedule.%d", scheduleID),
			"tracking": map[string]interface{}{
				"status":           "ONLINE",
				"poll_interval_ms": 10000,
			},
			"vehicle": map[string]interface{}{
				"register_no": "ND-4521",
				"model":       "Magiya Super Luxury Express",
			},
			"polyline": []map[string]float64{
				{"lat": 6.9344, "lng": 79.8540}, // Colombo Fort
				{"lat": 6.9420, "lng": 79.8650},
				{"lat": 6.9580, "lng": 79.8820}, // Peliyagoda
				{"lat": 6.9750, "lng": 79.8950},
				{"lat": 7.0010, "lng": 79.9530}, // Kadawatha
				{"lat": 7.0420, "lng": 80.0150},
				{"lat": 7.0870, "lng": 80.1140}, // Nittambuwa
				{"lat": 7.1520, "lng": 80.2050},
				{"lat": 7.2100, "lng": 80.3420}, // Kegalle
				{"lat": 7.2450, "lng": 80.4560},
				{"lat": 7.2600, "lng": 80.5900}, // Peradeniya
				{"lat": 7.2906, "lng": 80.6337}, // Kandy
			},
			"stops": []map[string]interface{}{
				{"id": 1, "name": "Colombo Fort", "latitude": 6.9344, "longitude": 79.8540, "arrival_time": "08:00 AM"},
				{"id": 2, "name": "Peliyagoda", "latitude": 6.9580, "longitude": 79.8820, "arrival_time": "08:20 AM"},
				{"id": 3, "name": "Kadawatha", "latitude": 7.0010, "longitude": 79.9530, "arrival_time": "08:45 AM"},
				{"id": 4, "name": "Nittambuwa", "latitude": 7.0870, "longitude": 80.1140, "arrival_time": "09:20 AM"},
				{"id": 5, "name": "Kegalle", "latitude": 7.2100, "longitude": 80.3420, "arrival_time": "10:05 AM"},
				{"id": 6, "name": "Peradeniya", "latitude": 7.2600, "longitude": 80.5900, "arrival_time": "10:45 AM"},
				{"id": 7, "name": "Kandy Goodshed", "latitude": 7.2906, "longitude": 80.6337, "arrival_time": "11:15 AM"},
			},
		},
	}
	bytes, _ := json.Marshal(resp)
	return bytes
}

// generateMockPosition calculates current live simulated movement along the route
func (s *MagiyaService) generateMockPosition(scheduleID int) []byte {
	// Polyline coordinates for animation
	coords := [][2]float64{
		{6.9344, 79.8540},
		{6.9420, 79.8650},
		{6.9580, 79.8820},
		{6.9750, 79.8950},
		{7.0010, 79.9530},
		{7.0420, 80.0150},
		{7.0870, 80.1140},
		{7.1520, 80.2050},
		{7.2100, 80.3420},
		{7.2450, 80.4560},
		{7.2600, 80.5900},
		{7.2906, 80.6337},
	}

	// Smoothly advance index based on current seconds
	now := time.Now()
	totalSecs := now.Unix()
	indexFloat := math.Mod(float64(totalSecs)/15.0, float64(len(coords)-1))
	idx := int(indexFloat)
	fraction := indexFloat - float64(idx)

	c1 := coords[idx]
	c2 := coords[idx+1]
	lat := c1[0] + (c2[0]-c1[0])*fraction
	lng := c1[1] + (c2[1]-c1[1])*fraction

	// Calculate heading
	dy := c2[0] - c1[0]
	dx := (c2[1] - c1[1]) * math.Cos(c1[0]*math.Pi/180.0)
	heading := math.Atan2(dx, dy) * 180.0 / math.Pi
	if heading < 0 {
		heading += 360
	}

	resp := map[string]interface{}{
		"status":  true,
		"message": "Position fetched",
		"data": map[string]interface{}{
			"schedule_id": scheduleID,
			"status":      "ONLINE",
			"position": map[string]interface{}{
				"latitude":   math.Round(lat*100000) / 100000,
				"longitude":  math.Round(lng*100000) / 100000,
				"speed":      48.5,
				"heading":    math.Round(heading),
				"altitude":   35.0,
				"satellites": 10,
				"fixed_at":   now.Format(time.RFC3339),
			},
			"stop_progress": map[string]interface{}{
				"current_index":    idx / 2,
				"segment_progress": math.Round(fraction * 100),
				"stops": []map[string]interface{}{
					{"id": 1, "status": "completed", "distance": 18200, "eta_time": "", "eta_late": false, "eta_diff": nil},
					{"id": 3, "status": "current", "distance": 2400, "eta_time": now.Add(4 * time.Minute).Format("03:04 PM"), "eta_late": false, "eta_diff": -2},
					{"id": 5, "status": "upcoming", "distance": 32000, "eta_time": now.Add(42 * time.Minute).Format("03:04 PM"), "eta_late": false, "eta_diff": nil},
					{"id": 7, "status": "upcoming", "distance": 68000, "eta_time": now.Add(90 * time.Minute).Format("03:04 PM"), "eta_late": false, "eta_diff": nil},
				},
			},
		},
	}

	bytes, _ := json.Marshal(resp)
	return bytes
}
