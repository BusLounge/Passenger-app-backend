package services

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
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
	cachedStations []byte
	parsedStations []MagiyaStation
	cacheExpiry    time.Time
	
	schedulesCache map[string]scheduleCacheEntry

	mu             sync.RWMutex
}

// NewMagiyaService creates a new Magiya service
func NewMagiyaService(logger *logrus.Logger) *MagiyaService {
	return &MagiyaService{
		logger:         logger,
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
