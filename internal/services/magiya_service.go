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

// MagiyaService handles interactions with the Magiya Merchant API
type MagiyaService struct {
	logger         *logrus.Logger
	cachedStations []byte
	cacheExpiry    time.Time
	mu             sync.RWMutex
}

// NewMagiyaService creates a new Magiya service
func NewMagiyaService(logger *logrus.Logger) *MagiyaService {
	return &MagiyaService{
		logger: logger,
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
