package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// scheduleCacheEntry stores the JSON response and expiration time
type scheduleCacheEntry struct {
	data   []byte
	expiry time.Time
}

// MagiyaService handles interactions with the Magiya Merchant API
type MagiyaService struct {
	logger         *logrus.Logger
	cachedStations []byte
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
