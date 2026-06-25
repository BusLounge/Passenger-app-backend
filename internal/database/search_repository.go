package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// SearchRepository handles database operations for search functionality
type SearchRepository struct {
	db *sqlx.DB
}

// NewSearchRepository creates a new search repository
func NewSearchRepository(db DB) *SearchRepository {
	// Type assertion to get *sqlx.DB
	if postgresDB, ok := db.(*PostgresDB); ok {
		return &SearchRepository{db: postgresDB.DB}
	}
	// Fallback - this should not happen in practice
	panic("Invalid database type for SearchRepository")
}

// FindStaticLoungeRoutes finds lounge-to-lounge connections without requiring scheduled trips
// This is used when no active schedules exist but we want to show route availability
func (r *SearchRepository) FindStaticLoungeRoutes(
	fromLat, fromLng float64,
	toLat, toLng float64,
	radiusMeters float64,
) ([]models.TripResult, error) {

	const query = `
WITH
haversine AS (SELECT 6371000.0 AS erm),

start_lounges AS (
    SELECT l.id, l.lounge_name,
           erm * 2 * ASIN(SQRT(
               POWER(SIN(RADIANS((l.latitude - $1)/2)),2) +
               COS(RADIANS($1))*COS(RADIANS(l.latitude))*
               POWER(SIN(RADIANS((l.longitude - $2)/2)),2)
           )) AS dist_m
    FROM lounges l, haversine
    WHERE l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND erm * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude - $1)/2)),2) +
              COS(RADIANS($1))*COS(RADIANS(l.latitude))*
              POWER(SIN(RADIANS((l.longitude - $2)/2)),2)
          )) <= $5
    ORDER BY dist_m ASC
    LIMIT 5
),

drop_lounges AS (
    SELECT l.id, l.lounge_name,
           erm * 2 * ASIN(SQRT(
               POWER(SIN(RADIANS((l.latitude - $3)/2)),2) +
               COS(RADIANS($3))*COS(RADIANS(l.latitude))*
               POWER(SIN(RADIANS((l.longitude - $4)/2)),2)
           )) AS dist_m
    FROM lounges l, haversine
    WHERE l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND erm * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude - $3)/2)),2) +
              COS(RADIANS($3))*COS(RADIANS(l.latitude))*
              POWER(SIN(RADIANS((l.longitude - $4)/2)),2)
          )) <= $5
    ORDER BY dist_m ASC
    LIMIT 5
),

lounge_pairs AS (
    SELECT sl.id AS sl_id, sl.lounge_name AS sl_name, sl.dist_m AS sl_dist,
           dl.id AS dl_id, dl.lounge_name AS dl_name, dl.dist_m AS dl_dist,
           mr.id AS route_id, mr.route_name, mr.route_number,
           mr.origin_city AS main_route_origin, mr.destination_city AS main_route_destination
    FROM start_lounges sl
    CROSS JOIN drop_lounges dl
    JOIN lounge_routes lr_s ON lr_s.lounge_id = sl.id
    JOIN lounge_routes lr_d ON lr_d.lounge_id = dl.id
                           AND lr_d.master_route_id = lr_s.master_route_id
    JOIN master_route_stops mrs_s ON mrs_s.id = lr_s.stop_before_id
    JOIN master_route_stops mrs_d ON mrs_d.id = lr_d.stop_before_id
    JOIN master_routes mr ON mr.id = lr_s.master_route_id AND mr.is_active = true
    WHERE mrs_s.stop_order < mrs_d.stop_order AND sl.id <> dl.id
    ORDER BY sl.dist_m ASC, dl.dist_m ASC
    LIMIT 10
)

SELECT lp.sl_name AS start_lounge_name, lp.dl_name AS drop_lounge_name,
       lp.sl_dist AS start_dist_m, lp.dl_dist AS drop_dist_m,
       lp.route_name, lp.route_number, lp.route_id::text AS master_route_id,
       lp.main_route_origin, lp.main_route_destination
FROM lounge_pairs lp
ORDER BY lp.sl_dist ASC, lp.dl_dist ASC
LIMIT 5;
`

	type staticRow struct {
		StartLoungeName      string  `db:"start_lounge_name"`
		DropLoungeName       string  `db:"drop_lounge_name"`
		StartDistM           float64 `db:"start_dist_m"`
		DropDistM            float64 `db:"drop_dist_m"`
		RouteName            string  `db:"route_name"`
		RouteNumber          *string `db:"route_number"`
		MasterRouteID        string  `db:"master_route_id"`
		MainRouteOrigin      *string `db:"main_route_origin"`
		MainRouteDestination *string `db:"main_route_destination"`
	}

	var rows []staticRow
	if err := r.db.Select(&rows, query,
		fromLat, fromLng, toLat, toLng, radiusMeters,
	); err != nil {
		return nil, fmt.Errorf("FindStaticLoungeRoutes: %w", err)
	}

	results := make([]models.TripResult, 0, len(rows))
	for _, row := range rows {
		// Create a ghost trip with no actual bus schedule
		ghostTripID := uuid.New() // Generate a fake ID for the ghost trip
		startL := row.StartLoungeName
		dropL := row.DropLoungeName

		results = append(results, models.TripResult{
			TripID:           ghostTripID,
			RouteName:        row.RouteName,
			RouteNumber:      row.RouteNumber,
			BusType:          "Unknown",                 // No bus assigned
			DepartureTime:    time.Now(),                // Placeholder
			EstimatedArrival: time.Now().Add(time.Hour), // Placeholder
			DurationMinutes:  60,                        // Placeholder
			TotalSeats:       0,                         // No seats available
			Fare:             0.0,                       // No fare
			BoardingPoint:    startL,                    // Use lounge name as boarding point
			DroppingPoint:    dropL,                     // Use lounge name as dropping point
			FromLounge:       &startL,
			ToLounge:         &dropL,
			FromLoungeDistKm: row.StartDistM / 1000.0,
			ToLoungeDistKm:   row.DropDistM / 1000.0,
			BusFeatures: models.BusFeatures{
				HasWiFi:          false,
				HasAC:            false,
				HasChargingPorts: false,
				HasEntertainment: false,
				HasRefreshments:  false,
			},
			IsBookable:           false,                // Cannot book ghost trips
			RouteStops:           []models.RouteStop{}, // No route stops
			MasterRouteID:        &row.MasterRouteID,
			MainRouteOrigin:      row.MainRouteOrigin,
			MainRouteDestination: row.MainRouteDestination,
			IsTransit:            false,
			// New fields for route discovery
			DiscoveryStatus:    "route_available",
			RemainingGapKm:     0.0,
			RouteExists:        true,
			HasActiveSchedules: false, // This is the key flag for ghost trips
		})
	}

	return results, nil
}

// StopPairResult holds the result of finding two stops on the same route
type StopPairResult struct {
	FromStop    *models.StopInfo
	ToStop      *models.StopInfo
	FromID      uuid.UUID
	ToID        uuid.UUID
	RouteID     uuid.UUID
	RouteName   string
	RouteNumber string
	Matched     bool
	DistanceKm  float64 // km to nearest join stop (only set for intercept results)
	DistanceStr string  // human-readable e.g. "2.3 km away"
}

// FindStopPairOnSameRoute finds two stops that are on the same route with fuzzy matching
// This ensures both stops can be connected by a trip
func (r *SearchRepository) FindStopPairOnSameRoute(fromName, toName string) (*StopPairResult, error) {
	query := `
		SELECT
			from_stop.id as from_id,
			from_stop.stop_name as from_name,
			to_stop.id as to_id,
			to_stop.stop_name as to_name,
			from_stop.master_route_id as route_id,
			mr.route_name,
			mr.route_number,
			from_stop.stop_order as from_order,
			to_stop.stop_order as to_order
		FROM master_route_stops from_stop
		INNER JOIN master_route_stops to_stop
			ON from_stop.master_route_id = to_stop.master_route_id
		INNER JOIN master_routes mr ON mr.id = from_stop.master_route_id
		WHERE
			LOWER(from_stop.stop_name) LIKE LOWER('%' || $1 || '%')
			AND LOWER(to_stop.stop_name) LIKE LOWER('%' || $2 || '%')
			AND from_stop.stop_order < to_stop.stop_order
			AND mr.is_active = true
		ORDER BY
			-- Exact stop name match first
			CASE WHEN LOWER(from_stop.stop_name) = LOWER($1) THEN 0 ELSE 1 END,
			CASE WHEN LOWER(to_stop.stop_name) = LOWER($2) THEN 0 ELSE 1 END,
			-- Prefer routes whose name or destination city contains the destination term.
			-- This ensures "Colombo - Kandy" beats "Colombo - Batticaloa" when searching for Kandy,
			-- because both routes share a "Kandy" stop but only the first is a Kandy route.
			CASE WHEN LOWER(mr.route_name) LIKE LOWER('%' || $2 || '%')
			          OR LOWER(mr.destination_city) LIKE LOWER('%' || $2 || '%')
			     THEN 0 ELSE 1 END,
			-- Also prefer routes whose origin matches from-term
			CASE WHEN LOWER(mr.route_name) LIKE LOWER('%' || $1 || '%')
			          OR LOWER(mr.origin_city) LIKE LOWER('%' || $1 || '%')
			     THEN 0 ELSE 1 END,
			-- Fall back to route with most stops (highest coverage)
			(SELECT COUNT(*) FROM master_route_stops WHERE master_route_id = mr.id) DESC
		LIMIT 1
	`

	var result struct {
		FromID      uuid.UUID `db:"from_id"`
		FromName    string    `db:"from_name"`
		ToID        uuid.UUID `db:"to_id"`
		ToName      string    `db:"to_name"`
		RouteID     uuid.UUID `db:"route_id"`
		RouteName   string    `db:"route_name"`
		RouteNumber string    `db:"route_number"`
		FromOrder   int       `db:"from_order"`
		ToOrder     int       `db:"to_order"`
	}

	err := r.db.Get(&result, query, strings.TrimSpace(fromName), strings.TrimSpace(toName))
	if err != nil {
		if err == sql.ErrNoRows {
			// No stop pair found on same route
			return &StopPairResult{
				FromStop: &models.StopInfo{
					Matched:       false,
					OriginalInput: fromName,
				},
				ToStop: &models.StopInfo{
					Matched:       false,
					OriginalInput: toName,
				},
				Matched: false,
			}, nil
		}
		return nil, fmt.Errorf("error finding stop pair: %w", err)
	}

	// Successfully found both stops on the same route
	return &StopPairResult{
		FromStop: &models.StopInfo{
			ID:            &result.FromID,
			Name:          result.FromName,
			Matched:       true,
			OriginalInput: fromName,
		},
		ToStop: &models.StopInfo{
			ID:            &result.ToID,
			Name:          result.ToName,
			Matched:       true,
			OriginalInput: toName,
		},
		FromID:      result.FromID,
		ToID:        result.ToID,
		RouteID:     result.RouteID,
		RouteName:   result.RouteName,
		RouteNumber: result.RouteNumber,
		Matched:     true,
	}, nil
}

// ResolveCoordinates finds geographic coordinates first checking DB, then checking external Open-Meteo API
func (r *SearchRepository) ResolveCoordinates(locationName string) (float64, float64, error) {
	// First check database
	var lat, lng float64
	query := `
		SELECT latitude, longitude
		FROM master_route_stops
		WHERE LOWER(stop_name) LIKE LOWER('%' || $1 || '%')
		   OR LOWER($1) LIKE LOWER('%' || stop_name || '%')
		ORDER BY is_major_stop DESC LIMIT 1
	`
	err := r.db.QueryRow(query, strings.TrimSpace(locationName)).Scan(&lat, &lng)
	if err == nil && lat != 0 && lng != 0 {
		return lat, lng, nil
	}

	// Fallback to Open-Meteo Geocoding API (Fast and Free)
	apiURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&format=json", url.QueryEscape(strings.TrimSpace(locationName)))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("geocoding api error: %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Lat     float64 `json:"latitude"`
			Lon     float64 `json:"longitude"`
			Country string  `json:"country"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	if len(result.Results) == 0 {
		return 0, 0, fmt.Errorf("location not found")
	}

	// Prefer Sri Lanka if there are multiple global results, otherwise take first
	for _, r := range result.Results {
		if r.Country == "Sri Lanka" {
			return r.Lat, r.Lon, nil
		}
	}

	return result.Results[0].Lat, result.Results[0].Lon, nil
}

// FindInterceptStopPair implements intelligent nearby-bus-stop discovery.
//
// When the user searches B→C but no direct bus exists:
//  1. Resolves coordinates for user's "From" location B (via DB or external Geocoding)
//  2. Find routes whose destination_city or route_name matches "To" (C)
//  3. For each such route, look up bus_owner_routes.selected_stop_ids
//  4. Resolve all those stop IDs to (id, stop_name, lat, lng) from master_route_stops
//  5. Find the stop in that list geographically nearest to location B coordinates
//  6. Return that stop as the boarding point + the destination stop as the alighting point
func (r *SearchRepository) FindInterceptStopPair(fromName, toName string, fromLat, fromLng, toLat, toLng *float64) (*StopPairResult, error) {
	var userLat, userLng float64
	var destLat, destLng float64
	var err error

	// Resolve From coordinates
	if fromLat != nil && fromLng != nil {
		userLat = *fromLat
		userLng = *fromLng
	} else {
		userLat, userLng, err = r.ResolveCoordinates(fromName)
		if err != nil {
			return &StopPairResult{Matched: false}, nil
		}
	}

	// Resolve To coordinates
	if toLat != nil && toLng != nil {
		destLat = *toLat
		destLng = *toLng
	} else {
		destLat, destLng, err = r.ResolveCoordinates(toName)
		if err != nil {
			return &StopPairResult{Matched: false}, nil
		}
	}

	query := `
		WITH
		-- Step 1: find destination stops near destination coordinates
		destination_stops AS (
			SELECT mrs.id       AS dest_id,
			       mrs.stop_name AS dest_name,
			       mrs.stop_order AS dest_order,
			       mrs.master_route_id,
				   -- Distance to destination city center
				   SQRT(POWER(mrs.latitude - $3, 2) + POWER(mrs.longitude - $4, 2)) as dest_dist
			FROM   master_route_stops mrs
			WHERE  mrs.latitude IS NOT NULL AND mrs.longitude IS NOT NULL
			-- Within 20km of destination city (Kandy/Jaffna etc)
			AND SQRT(POWER((mrs.latitude - $3) * 111.0, 2) + POWER((mrs.longitude - $4) * 111.0 * COS(RADIANS($3)), 2)) < 20.0
		),

		-- Step 2: find routes that contain ANY of these destination stops
		matching_routes AS (
			SELECT DISTINCT mr.id AS master_route_id,
			       mr.route_name,
				   mr.route_number
			FROM   master_routes mr
			JOIN   destination_stops ds ON ds.master_route_id = mr.id
			WHERE  mr.is_active = true
		),

		-- Step 3: Candidate boarding stops on these matching routes
		candidate_stops AS (
			SELECT DISTINCT
			       mrs.id,
			       mrs.stop_name,
			       mrs.stop_order,
			       mrs.latitude,
			       mrs.longitude,
			       mrs.is_major_stop,
			       mrs.master_route_id
			FROM   master_route_stops mrs
			JOIN   matching_routes mr ON mr.master_route_id = mrs.master_route_id
			WHERE  mrs.latitude  IS NOT NULL
			  AND  mrs.longitude IS NOT NULL
		)

		-- Step 4: score candidate stops by distance to user location
		SELECT
			cs.id                                                                AS from_id,
			cs.stop_name                                                         AS from_name,
			ds.dest_id                                                           AS to_id,
			ds.dest_name                                                         AS to_name,
			cs.master_route_id                                                   AS route_id,
			mr.route_name,
			-- Approximate Euclidean distance (fine for nearby-stop detection)
			SQRT(
			  POWER(cs.latitude  - $1,  2) +
			  POWER(cs.longitude - $2, 2)
			)                                                                    AS distance,
			-- Convert degree-distance to km (rough: 1° ≈ 111 km)
			SQRT(
			  POWER((cs.latitude  - $1)  * 111.0, 2) +
			  POWER((cs.longitude - $2) * 111.0 * COS(RADIANS($1)), 2)
			)                                                                    AS distance_km,
			mr.route_number
		FROM   candidate_stops cs
		JOIN   destination_stops ds  ON ds.master_route_id = cs.master_route_id
		JOIN   matching_routes   mr  ON mr.master_route_id = cs.master_route_id
		-- Ensure boarding stop comes before destination stop on the route
		WHERE  cs.stop_order < ds.dest_order
		ORDER  BY distance ASC, ds.dest_dist ASC
		LIMIT  1
	`

	var result struct {
		FromID      uuid.UUID `db:"from_id"`
		FromName    string    `db:"from_name"`
		ToID        uuid.UUID `db:"to_id"`
		ToName      string    `db:"to_name"`
		RouteID     uuid.UUID `db:"route_id"`
		RouteName   string    `db:"route_name"`
		RouteNumber string    `db:"route_number"`
		Distance    float64   `db:"distance"`
		DistanceKm  float64   `db:"distance_km"`
	}

	err = r.db.Get(&result, query, userLat, userLng, destLat, destLng)
	if err != nil {
		if err == sql.ErrNoRows {
			return &StopPairResult{Matched: false}, nil
		}
		return nil, fmt.Errorf("error finding intercept stop pair: %w", err)
	}

	distanceStr := fmt.Sprintf("%.1f km away", result.DistanceKm)

	return &StopPairResult{
		FromStop: &models.StopInfo{
			ID:            &result.FromID,
			Name:          result.FromName,
			Matched:       true,
			OriginalInput: fromName,
		},
		ToStop: &models.StopInfo{
			ID:            &result.ToID,
			Name:          result.ToName,
			Matched:       true,
			OriginalInput: toName,
		},
		FromID:      result.FromID,
		ToID:        result.ToID,
		RouteID:     result.RouteID,
		RouteName:   result.RouteName,
		RouteNumber: result.RouteNumber,
		Matched:     true,
		DistanceKm:  result.DistanceKm,
		DistanceStr: distanceStr,
	}, nil
}

// FindDirectTrips finds all direct trips between two stops

func (r *SearchRepository) FindDirectTrips(
	fromStopID, toStopID uuid.UUID,
	afterTime time.Time,
	limit int,
) ([]models.TripResult, error) {
	// Log search parameters
	fmt.Printf("\n🔍 === SEARCH QUERY DEBUG ===\n")
	fmt.Printf("FROM Stop ID: %s\n", fromStopID.String())
	fmt.Printf("TO Stop ID: %s\n", toStopID.String())
	fmt.Printf("After Time: %s\n", afterTime.Format(time.RFC3339))
	fmt.Printf("Limit: %d\n", limit)

	// Check how many scheduled trips exist that match basic criteria
	var debugCounts struct {
		TotalTrips    int `db:"total_trips"`
		BookableTrips int `db:"bookable_trips"`
		FutureTrips   int `db:"future_trips"`
		ValidStatus   int `db:"valid_status"`
		WithBORRoute  int `db:"with_bor_route"`
	}
	debugQuery := `
		SELECT 
			COUNT(*) as total_trips,
			COUNT(*) FILTER (WHERE is_bookable = true) as bookable_trips,
			COUNT(*) FILTER (WHERE departure_datetime > ($1 AT TIME ZONE 'Asia/Colombo')) as future_trips,
			COUNT(*) FILTER (WHERE status IN ('scheduled', 'confirmed')) as valid_status,
			COUNT(*) FILTER (WHERE bus_owner_route_id IS NOT NULL) as with_bor_route
		FROM scheduled_trips
	`
	if err := r.db.Get(&debugCounts, debugQuery, afterTime); err == nil {
		fmt.Printf("📊 Scheduled Trips Stats:\n")
		fmt.Printf("   Total: %d | Bookable: %d | Future: %d | Valid Status: %d | With Custom Route: %d\n",
			debugCounts.TotalTrips, debugCounts.BookableTrips, debugCounts.FutureTrips,
			debugCounts.ValidStatus, debugCounts.WithBORRoute)
	}

	query := `
		SELECT DISTINCT ON (st.id)
			st.id as trip_id,
			COALESCE(bor.custom_route_name, mr_bor.route_name, mr_permit.route_name) as route_name,
			COALESCE(mr_bor.route_number, mr_permit.route_number) as route_number,
			b.bus_type,
			st.departure_datetime as departure_time,
			-- Calculate arrival time: departure + duration
			st.departure_datetime +
				(COALESCE(st.estimated_duration_minutes, 0) * interval '1 minute') as estimated_arrival,
			COALESCE(st.estimated_duration_minutes, 0) as duration_minutes,
			-- Available seats removed - will be calculated from separate booking table
			COALESCE(bslt.total_seats, 0) as total_seats,
			COALESCE(rp.approved_fare, st.base_fare, 0) as fare,
			from_stop.stop_name as boarding_point,
			to_stop.stop_name as dropping_point,
			COALESCE(b.has_wifi, false) as has_wifi,
			COALESCE(b.has_ac, false) as has_ac,
			COALESCE(b.has_charging_ports, false) as has_charging_ports,
			COALESCE(b.has_entertainment, false) as has_entertainment,
			COALESCE(b.has_refreshments, false) as has_refreshments,
			st.is_bookable,
			-- Route info for fetching stops
			bor.id as bus_owner_route_id,
			-- Use bor.master_route_id first, fall back to permit's master_route_id
			COALESCE(bor.master_route_id, rp.master_route_id)::text as master_route_id,
			-- Fetch main route cities from master_routes
			COALESCE(mr_bor.origin_city, mr_permit.origin_city) as main_route_origin,
			COALESCE(mr_bor.destination_city, mr_permit.destination_city) as main_route_destination
		FROM scheduled_trips st
		-- Join bus owner route
		LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
		-- Route name/number via bus_owner_route path
		LEFT JOIN master_routes mr_bor ON bor.master_route_id = mr_bor.id
		-- Join permit for fare and bus
		LEFT JOIN route_permits rp ON st.permit_id = rp.id
		-- Route name/number fallback via permit path (used when bus_owner_route_id is NULL)
		LEFT JOIN master_routes mr_permit ON rp.master_route_id = mr_permit.id
		LEFT JOIN buses b ON rp.bus_registration_number = b.license_plate
		-- Join seat layout template to get total_seats
		LEFT JOIN bus_seat_layout_templates bslt ON b.seat_layout_id = bslt.id
		-- Get stop information
		JOIN master_route_stops from_stop ON from_stop.id = $1
		JOIN master_route_stops to_stop ON to_stop.id = $2
		-- Verify stops are on the same route as the trip
		-- COALESCE: prefer bus_owner_route's master_route_id, fall back to permit's master_route_id
		JOIN master_route_stops check_from ON
			check_from.master_route_id = COALESCE(bor.master_route_id, rp.master_route_id)
			AND check_from.id = $1
		JOIN master_route_stops check_to ON
			check_to.master_route_id = COALESCE(bor.master_route_id, rp.master_route_id)
			AND check_to.id = $2
		WHERE
			-- Trip must be bookable and in valid status
			st.is_bookable = true
			AND st.status IN ('scheduled', 'confirmed')
			-- Departure must be in the future
			AND st.departure_datetime > ($3 AT TIME ZONE 'Asia/Colombo')
			-- Stops must be in correct order
			AND check_from.stop_order < check_to.stop_order
			-- For bus owner routes, check if stops are selected
			-- If selected_stop_ids is NULL or empty, treat as "all stops available"
			AND (
				bor.id IS NULL
				OR bor.selected_stop_ids IS NULL
				OR array_length(bor.selected_stop_ids, 1) IS NULL
				OR array_length(bor.selected_stop_ids, 1) = 0
				OR (
					$1 = ANY(bor.selected_stop_ids)
					AND $2 = ANY(bor.selected_stop_ids)
				)
			)
		ORDER BY st.id, st.departure_datetime
		LIMIT $4
	`

	// Use intermediate struct to scan flat SQL results
	type tripWithFeatures struct {
		TripID           uuid.UUID `db:"trip_id"`
		RouteName        string    `db:"route_name"`
		RouteNumber      *string   `db:"route_number"`
		BusType          *string   `db:"bus_type"` // Nullable - bus might not have type set
		DepartureTime    time.Time `db:"departure_time"`
		EstimatedArrival time.Time `db:"estimated_arrival"`
		DurationMinutes  int       `db:"duration_minutes"`
		TotalSeats       int       `db:"total_seats"`
		Fare             float64   `db:"fare"`
		BoardingPoint    string    `db:"boarding_point"`
		DroppingPoint    string    `db:"dropping_point"`
		HasWiFi          bool      `db:"has_wifi"`
		HasAC            bool      `db:"has_ac"`
		HasChargingPorts bool      `db:"has_charging_ports"`
		HasEntertainment bool      `db:"has_entertainment"`
		HasRefreshments  bool      `db:"has_refreshments"`
		IsBookable       bool      `db:"is_bookable"`
		// Route info for fetching stops
		BusOwnerRouteID      *string `db:"bus_owner_route_id"`
		MasterRouteID        *string `db:"master_route_id"`
		MainRouteOrigin      *string `db:"main_route_origin"`
		MainRouteDestination *string `db:"main_route_destination"`
	}

	var tempTrips []tripWithFeatures
	err := r.db.Select(&tempTrips, query, fromStopID, toStopID, afterTime, limit)
	if err != nil {
		fmt.Printf("❌ SQL Query Error: %v\n", err)
		return nil, fmt.Errorf("error finding trips: %w", err)
	}

	fmt.Printf("✅ SQL Query successful - Found %d trips\n", len(tempTrips))

	// If no trips found, run diagnostic query to see why
	if len(tempTrips) == 0 {
		fmt.Printf("\n⚠️  NO TRIPS FOUND - Running diagnostics...\n")

		type diagnostic struct {
			TripID             uuid.UUID `db:"trip_id"`
			Departure          time.Time `db:"departure"`
			IsBookable         bool      `db:"is_bookable"`
			Status             string    `db:"status"`
			IsFuture           bool      `db:"is_future"`
			HasBORRoute        bool      `db:"has_bor_route"`
			SelectedStopsCount *int      `db:"selected_stops_count"`
			FromInSelected     *bool     `db:"from_in_selected"`
			ToInSelected       *bool     `db:"to_in_selected"`
			StopsConnected     bool      `db:"stops_connected"`
		}

		diagQuery := `
			SELECT 
				st.id as trip_id,
				st.departure_datetime as departure,
				st.is_bookable,
				st.status,
				st.departure_datetime > ($3 AT TIME ZONE 'Asia/Colombo') as is_future,
				st.bus_owner_route_id IS NOT NULL as has_bor_route,
				CASE WHEN bor.id IS NOT NULL THEN array_length(bor.selected_stop_ids, 1) END as selected_stops_count,
				CASE WHEN bor.id IS NOT NULL AND bor.selected_stop_ids IS NOT NULL 
					THEN $1 = ANY(bor.selected_stop_ids) END as from_in_selected,
				CASE WHEN bor.id IS NOT NULL AND bor.selected_stop_ids IS NOT NULL 
					THEN $2 = ANY(bor.selected_stop_ids) END as to_in_selected,
				EXISTS (
					SELECT 1 
					FROM master_route_stops check_from
					JOIN master_route_stops check_to 
						ON check_from.master_route_id = check_to.master_route_id
					LEFT JOIN route_permits rp2 ON rp2.id = st.permit_id
					WHERE check_from.id = $1 
						AND check_to.id = $2
						AND check_from.stop_order < check_to.stop_order
						AND check_from.master_route_id = COALESCE(bor.master_route_id, rp2.master_route_id)
				) as stops_connected
			FROM scheduled_trips st
			LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
			WHERE st.departure_datetime > ($3 AT TIME ZONE 'Asia/Colombo') - INTERVAL '24 hours'
			ORDER BY st.departure_datetime
			LIMIT 10
		`

		var diags []diagnostic
		if err := r.db.Select(&diags, diagQuery, fromStopID, toStopID, afterTime); err == nil {
			for _, d := range diags {
				reasons := []string{}
				if !d.IsBookable {
					reasons = append(reasons, "❌ NOT BOOKABLE")
				}
				if d.Status != "scheduled" && d.Status != "confirmed" {
					reasons = append(reasons, fmt.Sprintf("❌ WRONG STATUS: %s", d.Status))
				}
				if !d.IsFuture {
					reasons = append(reasons, "❌ PAST DEPARTURE")
				}
				// Check selected_stop_ids for custom routes
				if d.HasBORRoute {
					if d.SelectedStopsCount == nil || *d.SelectedStopsCount == 0 {
						reasons = append(reasons, "⚠️  selected_stop_ids is NULL/empty (now allowed)")
					} else if d.FromInSelected != nil && !*d.FromInSelected {
						reasons = append(reasons, "❌ FROM STOP NOT IN selected_stop_ids")
					} else if d.ToInSelected != nil && !*d.ToInSelected {
						reasons = append(reasons, "❌ TO STOP NOT IN selected_stop_ids")
					}
				}
				if !d.StopsConnected {
					reasons = append(reasons, "❌ STOPS NOT ON SAME ROUTE OR WRONG ORDER")
				}

				if len(reasons) == 0 {
					reasons = append(reasons, "✅ Should have matched!")
				}

				fmt.Printf("   Trip %s: %s | %s\n",
					d.TripID.String()[:8],
					d.Departure.Format("2006-01-02 15:04"),
					strings.Join(reasons, ", "))
			}
		}
		fmt.Printf("\n")
	}

	// Log each trip found for debugging
	for i, trip := range tempTrips {
		fmt.Printf("  Trip %d: %s (%s) - Departs: %s\n",
			i+1,
			trip.RouteName,
			trip.TripID.String()[:8],
			trip.DepartureTime.Format("2006-01-02 15:04"))
	}

	// Map to TripResult with nested BusFeatures
	trips := make([]models.TripResult, len(tempTrips))
	for i, temp := range tempTrips {
		// Handle nullable BusType - default to "Standard" if NULL
		busType := "Standard"
		if temp.BusType != nil {
			busType = *temp.BusType
		}

		trips[i] = models.TripResult{
			TripID:           temp.TripID,
			RouteName:        temp.RouteName,
			RouteNumber:      temp.RouteNumber,
			BusType:          busType,
			DepartureTime:    temp.DepartureTime,
			EstimatedArrival: temp.EstimatedArrival,
			DurationMinutes:  temp.DurationMinutes,
			TotalSeats:       temp.TotalSeats,
			Fare:             temp.Fare,
			BoardingPoint:    temp.BoardingPoint,
			DroppingPoint:    temp.DroppingPoint,
			BusFeatures: models.BusFeatures{
				HasWiFi:          temp.HasWiFi,
				HasAC:            temp.HasAC,
				HasChargingPorts: temp.HasChargingPorts,
				HasEntertainment: temp.HasEntertainment,
				HasRefreshments:  temp.HasRefreshments,
			},
			IsBookable:           temp.IsBookable,
			BusOwnerRouteID:      temp.BusOwnerRouteID,
			MasterRouteID:        temp.MasterRouteID,
			MainRouteOrigin:      temp.MainRouteOrigin,
			MainRouteDestination: temp.MainRouteDestination,
		}
	}

	return trips, nil
}

// LogSearch records a search query for analytics
func (r *SearchRepository) LogSearch(log *models.SearchLog) error {
	query := `
		INSERT INTO search_logs (
			from_input,
			to_input,
			from_stop_id,
			to_stop_id,
			results_count,
			response_time_ms,
			user_id,
			ip_address
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(
		query,
		log.FromInput,
		log.ToInput,
		log.FromStopID,
		log.ToStopID,
		log.ResultsCount,
		log.ResponseTimeMs,
		log.UserID,
		log.IPAddress,
	)

	if err != nil {
		return fmt.Errorf("error logging search: %w", err)
	}

	return nil
}

// GetPopularRoutes returns frequently searched routes
func (r *SearchRepository) GetPopularRoutes(limit int) ([]models.PopularRoute, error) {
	query := `
		SELECT
			from_input as from_stop_name,
			to_input as to_stop_name,
			COUNT(*) as search_count
		FROM search_logs
		WHERE from_stop_id IS NOT NULL
		  AND to_stop_id IS NOT NULL
		  AND created_at > NOW() - INTERVAL '30 days'
		GROUP BY from_input, to_input
		ORDER BY search_count DESC
		LIMIT $1
	`

	var routes []models.PopularRoute
	err := r.db.Select(&routes, query, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting popular routes: %w", err)
	}

	return routes, nil
}

// GetStopAutocomplete returns stop suggestions for autocomplete
func (r *SearchRepository) GetStopAutocomplete(searchTerm string, limit int) ([]models.StopAutocomplete, error) {
	query := `
		SELECT DISTINCT
			s.id as stop_id,
			s.stop_name,
			COUNT(DISTINCT s.master_route_id) as route_count
		FROM master_route_stops s
		JOIN master_routes r ON s.master_route_id = r.id
		WHERE LOWER(s.stop_name) LIKE LOWER($1)
		  AND r.is_active = true
		GROUP BY s.id, s.stop_name
		ORDER BY route_count DESC, s.stop_name
		LIMIT $2
	`

	searchPattern := "%" + strings.TrimSpace(searchTerm) + "%"

	var suggestions []models.StopAutocomplete
	err := r.db.Select(&suggestions, query, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting autocomplete suggestions: %w", err)
	}

	return suggestions, nil
}

// GetSearchAnalytics returns search analytics for admin dashboard
func (r *SearchRepository) GetSearchAnalytics(days int) (map[string]interface{}, error) {
	analytics := make(map[string]interface{})

	// Total searches
	var totalSearches int
	err := r.db.Get(&totalSearches, `
		SELECT COUNT(*)
		FROM search_logs
		WHERE created_at > NOW() - $1::INTERVAL
	`, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	analytics["total_searches"] = totalSearches

	// Average response time
	var avgResponseTime float64
	err = r.db.Get(&avgResponseTime, `
		SELECT COALESCE(AVG(response_time_ms), 0)
		FROM search_logs
		WHERE created_at > NOW() - $1::INTERVAL
	`, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	analytics["avg_response_time_ms"] = avgResponseTime

	// Success rate (searches with results)
	var successRate float64
	err = r.db.Get(&successRate, `
		SELECT COALESCE(
			100.0 * COUNT(CASE WHEN results_count > 0 THEN 1 END) / NULLIF(COUNT(*), 0),
			0
		)
		FROM search_logs
		WHERE created_at > NOW() - $1::INTERVAL
	`, fmt.Sprintf("%d days", days))
	if err != nil {
		return nil, err
	}
	analytics["success_rate"] = successRate

	return analytics, nil
}

// GetRouteStopsForTrip fetches the route stops for a trip based on bus_owner_route_id
// Returns stops ordered by stop_order for passenger to select boarding/alighting points
func (r *SearchRepository) GetRouteStopsForTrip(masterRouteID string, busOwnerRouteID *string) ([]models.RouteStop, error) {
	var stops []models.RouteStop

	if busOwnerRouteID != nil && *busOwnerRouteID != "" {
		// Bus owner has selected specific stops - only return those
		query := `
			SELECT 
				mrs.id,
				mrs.stop_name,
				mrs.stop_order,
				mrs.latitude,
				mrs.longitude,
				mrs.arrival_time_offset_minutes,
				mrs.is_major_stop
			FROM master_route_stops mrs
			JOIN bus_owner_routes bor ON bor.id = $1
			WHERE mrs.master_route_id = $2
			  AND mrs.id = ANY(bor.selected_stop_ids)
			ORDER BY mrs.stop_order ASC
		`
		err := r.db.Select(&stops, query, *busOwnerRouteID, masterRouteID)
		if err != nil {
			return nil, fmt.Errorf("error fetching route stops with bus owner route: %w", err)
		}
	} else {
		// No bus owner route - return all stops on master route
		query := `
			SELECT 
				id,
				stop_name,
				stop_order,
				latitude,
				longitude,
				arrival_time_offset_minutes,
				is_major_stop
			FROM master_route_stops
			WHERE master_route_id = $1
			ORDER BY stop_order ASC
		`
		err := r.db.Select(&stops, query, masterRouteID)
		if err != nil {
			return nil, fmt.Errorf("error fetching route stops: %w", err)
		}
	}

	return stops, nil
}
func (r *SearchRepository) FindTransitJourneys(fromName, toName string, fromLat, fromLng, toLat, toLng *float64, afterTime time.Time, limit int) ([]models.TripResult, error) {
	var userLat, userLng float64
	var destLat, destLng float64
	var err error

	// Resolve From coordinates
	if fromLat != nil && fromLng != nil {
		userLat = *fromLat
		userLng = *fromLng
	} else {
		userLat, userLng, err = r.ResolveCoordinates(fromName)
		if err != nil {
			return nil, nil // Silently fail if coordinates can't be resolved
		}
	}

	// Resolve To coordinates
	if toLat != nil && toLng != nil {
		destLat = *toLat
		destLng = *toLng
	} else {
		destLat, destLng, err = r.ResolveCoordinates(toName)
		if err != nil {
			return nil, nil
		}
	}

	// This query finds a transit hub (stop) that connects two routes
	query := `
		WITH 
		origin_routes AS (
			SELECT mrs.master_route_id, mrs.id as start_stop_id, mrs.stop_name as start_stop_name, mrs.stop_order as start_order
			FROM master_route_stops mrs
			WHERE mrs.latitude IS NOT NULL AND mrs.longitude IS NOT NULL
			AND SQRT(POWER((mrs.latitude - $1) * 111.0, 2) + POWER((mrs.longitude - $2) * 111.0 * COS(RADIANS($1)), 2)) < 15.0
		),
		destination_routes AS (
			SELECT mrs.master_route_id, mrs.id as end_stop_id, mrs.stop_name as end_stop_name, mrs.stop_order as end_order
			FROM master_route_stops mrs
			WHERE mrs.latitude IS NOT NULL AND mrs.longitude IS NOT NULL
			AND SQRT(POWER((mrs.latitude - $3) * 111.0, 2) + POWER((mrs.longitude - $4) * 111.0 * COS(RADIANS($3)), 2)) < 15.0
		),
		transit_hubs AS (
			SELECT 
				h1.stop_name as hub_name,
				h1.id as hub_id_leg1,
				h2.id as hub_id_leg2,
				oroutes.master_route_id as route1_id,
				droutes.master_route_id as route2_id,
				oroutes.start_stop_id,
				droutes.end_stop_id,
				oroutes.start_stop_name,
				droutes.end_stop_name
			FROM master_route_stops h1
			JOIN origin_routes oroutes ON oroutes.master_route_id = h1.master_route_id
			JOIN master_route_stops h2 ON h2.stop_name = h1.stop_name
			JOIN destination_routes droutes ON droutes.master_route_id = h2.master_route_id
			WHERE h1.stop_order > oroutes.start_order
			AND h2.stop_order < droutes.end_order
			AND h1.master_route_id != h2.master_route_id
			AND h1.is_major_stop = true
			LIMIT 5
		)
		SELECT hub_name, hub_id_leg1, hub_id_leg2, route1_id, route2_id, start_stop_id, end_stop_id, start_stop_name, end_stop_name FROM transit_hubs
	`

	rows, err := r.db.Queryx(query, userLat, userLng, destLat, destLng)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.TripResult
	for rows.Next() {
		var h struct {
			HubName       string    `db:"hub_name"`
			HubIDLeg1     uuid.UUID `db:"hub_id_leg1"`
			HubIDLeg2     uuid.UUID `db:"hub_id_leg2"`
			Route1ID      uuid.UUID `db:"route1_id"`
			Route2ID      uuid.UUID `db:"route2_id"`
			StartStopID   uuid.UUID `db:"start_stop_id"`
			EndStopID     uuid.UUID `db:"end_stop_id"`
			StartStopName string    `db:"start_stop_name"`
			EndStopName   string    `db:"end_stop_name"`
		}
		if err := rows.StructScan(&h); err != nil {
			continue
		}

		// Find trips for leg 1
		trips1, err := r.FindDirectTrips(h.StartStopID, h.HubIDLeg1, afterTime, 3)
		if err != nil || len(trips1) == 0 {
			trips1 = []models.TripResult{{
				TripID:           uuid.New(),
				RouteName:        "Route 1 to " + h.HubName,
				BoardingPoint:    h.StartStopName,
				DroppingPoint:    h.HubName,
				IsBookable:       false,
				BusType:          "Normal",
				DepartureTime:    afterTime,
				EstimatedArrival: afterTime.Add(2 * time.Hour),
			}}
		}

		// Find trips for leg 2
		for _, t1 := range trips1 {
			trips2, err := r.FindDirectTrips(h.HubIDLeg2, h.EndStopID, t1.EstimatedArrival.Add(30*time.Minute), 2)
			if err != nil || len(trips2) == 0 {
				trips2 = []models.TripResult{{
					TripID:           uuid.New(),
					RouteName:        "Route 2 from " + h.HubName,
					BoardingPoint:    h.HubName,
					DroppingPoint:    h.EndStopName,
					IsBookable:       false,
					BusType:          "Normal",
					DepartureTime:    t1.EstimatedArrival.Add(1 * time.Hour),
					EstimatedArrival: t1.EstimatedArrival.Add(4 * time.Hour),
				}}
			}

			for _, t2 := range trips2 {
				transitTrip := models.TripResult{
					TripID:           uuid.New(),
					RouteName:        fmt.Sprintf("via %s", h.HubName),
					IsTransit:        true,
					TransitPoint:     h.HubName,
					TransitPointID:   &h.HubIDLeg1,
					BoardingPoint:    t1.BoardingPoint,
					DroppingPoint:    t2.DroppingPoint,
					DepartureTime:    t1.DepartureTime,
					EstimatedArrival: t2.EstimatedArrival,
					DurationMinutes:  int(t2.EstimatedArrival.Sub(t1.DepartureTime).Minutes()),
					Fare:             t1.Fare + t2.Fare,
					Leg1:             &t1,
					Leg2:             &t2,
					IsBookable:       t1.IsBookable && t2.IsBookable,
				}
				results = append(results, transitTrip)
				if len(results) >= limit {
					return results, nil
				}
			}
		}
	}

	return results, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindLoungeDirectRoutes — Lounge-centric Direct Route Discovery
//
// Single PostgreSQL round-trip. Uses LATERAL JOINs so the nested fail-over
// (start-lounge × drop-lounge ranking + schedule validation) runs entirely
// on the database server.
//
// Parameters:
//
//	$1/$2  from_lat, from_lng  — origin coordinates
//	$3/$4  to_lat,   to_lng    — destination coordinates
//	$5     radius_metres       — candidate lounge search radius
//	$6     afterTime           — earliest acceptable departure (UTC)
//	$7     limit               — max rows returned
//
// Output includes from_lounge / to_lounge names so the Flutter card
// can prominently display the specific lounges found by the algorithm.
// ─────────────────────────────────────────────────────────────────────────────
func (r *SearchRepository) FindLoungeDirectRoutes(
	fromLat, fromLng float64,
	toLat, toLng float64,
	radiusMeters float64,
	afterTime time.Time,
	limit int,
) ([]models.TripResult, error) {

	if limit <= 0 {
		limit = 20
	}

	const loungeQuery = `
WITH
-- ── 1. Candidate lounges near the ORIGIN, ranked by Haversine distance ──────
-- LIMIT 1: Pin to the single nearest approved lounge to avoid result explosion
start_lounges AS (
    SELECT
        l.id           AS lounge_id,
        l.lounge_name,
        6371000 * 2 * ASIN(SQRT(
            POWER(SIN(RADIANS((l.latitude  - $1) / 2)), 2) +
            COS(RADIANS($1)) * COS(RADIANS(l.latitude)) *
            POWER(SIN(RADIANS((l.longitude - $2) / 2)), 2)
        )) AS dist_m
    FROM lounges l
    WHERE l.status = 'approved' AND l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND 6371000 * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude  - $1) / 2)), 2) +
              COS(RADIANS($1)) * COS(RADIANS(l.latitude)) *
              POWER(SIN(RADIANS((l.longitude - $2) / 2)), 2)
          )) <= $5
    ORDER BY dist_m ASC
    LIMIT 1
),

-- ── 2. Candidate lounges near the DESTINATION, ranked by proximity ───────────
-- LIMIT 1: Pin to the single nearest approved drop lounge
drop_lounges AS (
    SELECT
        l.id           AS lounge_id,
        l.lounge_name,
        6371000 * 2 * ASIN(SQRT(
            POWER(SIN(RADIANS((l.latitude  - $3) / 2)), 2) +
            COS(RADIANS($3)) * COS(RADIANS(l.latitude)) *
            POWER(SIN(RADIANS((l.longitude - $4) / 2)), 2)
        )) AS dist_m
    FROM lounges l
    WHERE l.status = 'approved' AND l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND 6371000 * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude  - $3) / 2)), 2) +
              COS(RADIANS($3)) * COS(RADIANS(l.latitude)) *
              POWER(SIN(RADIANS((l.longitude - $4) / 2)), 2)
          )) <= $5
    ORDER BY dist_m ASC
    LIMIT 1
),

-- ── 3. Directional lounge pairs sharing a master route ───────────────────────
--   Constraint: Ls.stop_order < Ld.stop_order  (directional validation)
matched_routes AS (
    SELECT
        sl.lounge_id          AS start_lounge_id,
        sl.lounge_name        AS start_lounge_name,
        sl.dist_m             AS start_dist_m,
        dl.lounge_id          AS drop_lounge_id,
        dl.lounge_name        AS drop_lounge_name,
        dl.dist_m             AS drop_dist_m,
        lr_s.master_route_id,
        mr.route_name,
        mr.route_number,
        mr.origin_city        AS main_route_origin,
        mr.destination_city   AS main_route_destination,
        lr_s.stop_before_id   AS boarding_stop_id,
        lr_d.stop_before_id   AS dropping_stop_id
    FROM start_lounges sl
    CROSS JOIN drop_lounges dl
    JOIN lounge_routes lr_s ON lr_s.lounge_id = sl.lounge_id
    JOIN lounge_routes lr_d ON lr_d.lounge_id = dl.lounge_id
                           AND lr_d.master_route_id = lr_s.master_route_id
    JOIN master_route_stops mrs_s ON mrs_s.id = lr_s.stop_before_id
    JOIN master_route_stops mrs_d ON mrs_d.id = lr_d.stop_before_id
    JOIN master_routes mr ON mr.id = lr_s.master_route_id AND mr.is_active = true
    WHERE mrs_s.stop_order < mrs_d.stop_order
      AND sl.lounge_id != dl.lounge_id
    ORDER BY sl.dist_m ASC, dl.dist_m ASC
)

-- ── 4. LATERAL: next available scheduled trip per lounge pair ─────────────────
SELECT
    mr_data.start_lounge_name,
    mr_data.drop_lounge_name,
    mr_data.route_name,
    mr_data.route_number,
    mr_data.start_dist_m,
    mr_data.drop_dist_m,
    mr_data.master_route_id::text,
    mr_data.main_route_origin,
    mr_data.main_route_destination,
    boarding_stop.stop_name   AS boarding_point,
    dropping_stop.stop_name   AS dropping_point,
    sched.trip_id,
    sched.departure_time,
    sched.estimated_arrival,
    sched.duration_minutes,
    sched.total_seats,
    sched.fare,
    sched.bus_type,
    sched.is_bookable,
    sched.has_wifi,
    sched.has_ac,
    sched.has_charging_ports,
    sched.has_entertainment,
    sched.has_refreshments,
    sched.bus_owner_route_id,
    sched.trip_master_route_id
FROM matched_routes mr_data
JOIN master_route_stops boarding_stop ON boarding_stop.id = mr_data.boarding_stop_id
JOIN master_route_stops dropping_stop ON dropping_stop.id = mr_data.dropping_stop_id
JOIN LATERAL (
    SELECT
        st.id::text                                                             AS trip_id,
        st.departure_datetime                                                   AS departure_time,
        st.departure_datetime +
            (COALESCE(st.estimated_duration_minutes,0) * INTERVAL '1 minute')  AS estimated_arrival,
        COALESCE(st.estimated_duration_minutes, 0)                              AS duration_minutes,
        COALESCE(bslt.total_seats, 0)                                           AS total_seats,
        COALESCE(rp.approved_fare, st.base_fare, 0)                             AS fare,
        COALESCE(b.bus_type, 'Normal')                                          AS bus_type,
        st.is_bookable,
        COALESCE(b.has_wifi,           false) AS has_wifi,
        COALESCE(b.has_ac,             false) AS has_ac,
        COALESCE(b.has_charging_ports, false) AS has_charging_ports,
        COALESCE(b.has_entertainment,  false) AS has_entertainment,
        COALESCE(b.has_refreshments,   false) AS has_refreshments,
        bor.id::text                                                             AS bus_owner_route_id,
        COALESCE(bor.master_route_id, rp.master_route_id)::text                AS trip_master_route_id
    FROM scheduled_trips st
    LEFT JOIN bus_owner_routes bor          ON bor.id = st.bus_owner_route_id
    LEFT JOIN route_permits rp              ON rp.id  = st.permit_id
    LEFT JOIN buses b                       ON b.license_plate = rp.bus_registration_number
    LEFT JOIN bus_seat_layout_templates bslt ON bslt.id = b.seat_layout_id
    WHERE COALESCE(bor.master_route_id, rp.master_route_id) = mr_data.master_route_id
      AND st.is_bookable = true
      AND st.status IN ('scheduled', 'confirmed')
      AND st.departure_datetime > $6
    ORDER BY st.departure_datetime ASC
) sched ON true
ORDER BY mr_data.start_dist_m ASC, mr_data.drop_dist_m ASC, sched.departure_time ASC
LIMIT $7
`

	type loungeRow struct {
		StartLoungeName   string    `db:"start_lounge_name"`
		DropLoungeName    string    `db:"drop_lounge_name"`
		RouteName         string    `db:"route_name"`
		RouteNumber       *string   `db:"route_number"`
		StartDistM        float64   `db:"start_dist_m"`
		DropDistM         float64   `db:"drop_dist_m"`
		MasterRouteID     string    `db:"master_route_id"`
		BoardingPoint     string    `db:"boarding_point"`
		DroppingPoint     string    `db:"dropping_point"`
		TripID            string    `db:"trip_id"`
		DepartureTime     time.Time `db:"departure_time"`
		EstimatedArrival  time.Time `db:"estimated_arrival"`
		DurationMinutes   int       `db:"duration_minutes"`
		TotalSeats        int       `db:"total_seats"`
		Fare              float64   `db:"fare"`
		BusType           string    `db:"bus_type"`
		IsBookable        bool      `db:"is_bookable"`
		HasWiFi           bool      `db:"has_wifi"`
		HasAC             bool      `db:"has_ac"`
		HasChargingPorts  bool      `db:"has_charging_ports"`
		HasEntertainment  bool      `db:"has_entertainment"`
		HasRefreshments   bool      `db:"has_refreshments"`
		BusOwnerRouteID   *string   `db:"bus_owner_route_id"`
		TripMasterRouteID *string   `db:"trip_master_route_id"`
	}

	var rows []loungeRow
	if err := r.db.Select(&rows, loungeQuery,
		fromLat, fromLng,
		toLat, toLng,
		radiusMeters,
		afterTime,
		limit,
	); err != nil {
		return nil, fmt.Errorf("FindLoungeDirectRoutes: %w", err)
	}

	trips := make([]models.TripResult, 0, len(rows))
	for _, row := range rows {
		tripID, _ := uuid.Parse(row.TripID)
		startLounge := row.StartLoungeName
		dropLounge := row.DropLoungeName
		trips = append(trips, models.TripResult{
			TripID:           tripID,
			RouteName:        row.RouteName,
			RouteNumber:      row.RouteNumber,
			BusType:          row.BusType,
			DepartureTime:    row.DepartureTime,
			EstimatedArrival: row.EstimatedArrival,
			DurationMinutes:  row.DurationMinutes,
			TotalSeats:       row.TotalSeats,
			Fare:             row.Fare,
			BoardingPoint:    row.BoardingPoint,
			DroppingPoint:    row.DroppingPoint,
			FromLounge:       &startLounge,
			ToLounge:         &dropLounge,
			FromLoungeDistKm: row.StartDistM / 1000.0,
			ToLoungeDistKm:   row.DropDistM / 1000.0,
			BusFeatures: models.BusFeatures{
				HasWiFi:          row.HasWiFi,
				HasAC:            row.HasAC,
				HasChargingPorts: row.HasChargingPorts,
				HasEntertainment: row.HasEntertainment,
				HasRefreshments:  row.HasRefreshments,
			},
			IsBookable:      row.IsBookable,
			BusOwnerRouteID: row.BusOwnerRouteID,
			MasterRouteID:   row.TripMasterRouteID,
		})
	}

	return trips, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindMultiModalRoutes — Unified Direct + One-Transit Discovery (single SQL round-trip)
//
// The query uses a layered CTE approach:
//  1. Haversine distance CTEs find candidate lounges near origin ($1/$2) and destination ($3/$4)
//     within $5 metres.
//  2. direct_pairs finds lounge pairs on the same directional master route.
//  3. transit_chains finds Ls→Lt→Ld chains (only evaluated when no direct pair exists,
//     enforced by NOT EXISTS (SELECT 1 FROM direct_pairs)).
//  4. Both are unified in a final UNION ALL ranked: direct first, then transit.
//  5. The 15-minute transfer buffer is enforced inside the Leg-2 LATERAL JOIN.
//
// ─────────────────────────────────────────────────────────────────────────────
func (r *SearchRepository) FindMultiModalRoutes(
	fromLat, fromLng float64,
	toLat, toLng float64,
	radiusMeters float64,
	afterTime time.Time,
	limit int,
) ([]models.TripResult, error) {

	if limit <= 0 {
		limit = 20
	}

	const mmQuery = `
WITH
haversine AS (SELECT 6371000.0 AS erm),

start_lounges AS (
    SELECT l.id, l.lounge_name,
           erm * 2 * ASIN(SQRT(
               POWER(SIN(RADIANS((l.latitude - $1)/2)),2) +
               COS(RADIANS($1))*COS(RADIANS(l.latitude))*
               POWER(SIN(RADIANS((l.longitude - $2)/2)),2)
           )) AS dist_m
    FROM lounges l, haversine
    WHERE l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND erm * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude - $1)/2)),2) +
              COS(RADIANS($1))*COS(RADIANS(l.latitude))*
              POWER(SIN(RADIANS((l.longitude - $2)/2)),2)
          )) <= $5
    ORDER BY dist_m ASC
),

drop_lounges AS (
    SELECT l.id, l.lounge_name,
           erm * 2 * ASIN(SQRT(
               POWER(SIN(RADIANS((l.latitude - $3)/2)),2) +
               COS(RADIANS($3))*COS(RADIANS(l.latitude))*
               POWER(SIN(RADIANS((l.longitude - $4)/2)),2)
           )) AS dist_m
    FROM lounges l, haversine
    WHERE l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND erm * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude - $3)/2)),2) +
              COS(RADIANS($3))*COS(RADIANS(l.latitude))*
              POWER(SIN(RADIANS((l.longitude - $4)/2)),2)
          )) <= $5
    ORDER BY dist_m ASC
),

direct_pairs AS (
    SELECT sl.id AS sl_id, sl.lounge_name AS sl_name, sl.dist_m AS sl_dist,
           dl.id AS dl_id, dl.lounge_name AS dl_name, dl.dist_m AS dl_dist,
           mr.id AS route_id, mr.route_name, mr.route_number,
           mr.origin_city AS main_route_origin, mr.destination_city AS main_route_destination,
           lr_s.stop_before_id AS boarding_stop_id,
           lr_d.stop_before_id AS dropping_stop_id
    FROM start_lounges sl
    CROSS JOIN drop_lounges dl
    JOIN lounge_routes lr_s ON lr_s.lounge_id = sl.id
    JOIN lounge_routes lr_d ON lr_d.lounge_id = dl.id
                           AND lr_d.master_route_id = lr_s.master_route_id
    JOIN master_route_stops mrs_s ON mrs_s.id = lr_s.stop_before_id
    JOIN master_route_stops mrs_d ON mrs_d.id = lr_d.stop_before_id
    JOIN master_routes mr ON mr.id = lr_s.master_route_id AND mr.is_active = true
    WHERE mrs_s.stop_order < mrs_d.stop_order AND sl.id <> dl.id
    ORDER BY sl.dist_m ASC, dl.dist_m ASC
    LIMIT 30
),

transit_chains_base AS (
    SELECT sl.id AS sl_id, sl.lounge_name AS sl_name, sl.dist_m AS sl_dist,
           dl.id AS dl_id, dl.lounge_name AS dl_name, dl.dist_m AS dl_dist,
           tl.id AS tl_id, tl.lounge_name AS tl_name,
           erm * 2 * ASIN(SQRT(
               POWER(SIN(RADIANS((tl.latitude - $1)/2)),2) +
               COS(RADIANS($1))*COS(RADIANS(tl.latitude))*
               POWER(SIN(RADIANS((tl.longitude - $2)/2)),2)
           )) +
           erm * 2 * ASIN(SQRT(
               POWER(SIN(RADIANS((tl.latitude - $3)/2)),2) +
               COS(RADIANS($3))*COS(RADIANS(tl.latitude))*
               POWER(SIN(RADIANS((tl.longitude - $4)/2)),2)
           )) AS hub_weight,
           lr1.master_route_id AS r1_id, lr2.master_route_id AS r2_id,
           lr1.stop_before_id AS b1_id, lrt1.stop_before_id AS d1_id,
           lrt2.stop_before_id AS b2_id, lr2.stop_before_id AS d2_id,
           DENSE_RANK() OVER(
               PARTITION BY sl.id, dl.id, lr1.master_route_id
               ORDER BY (
                   erm * 2 * ASIN(SQRT(
                       POWER(SIN(RADIANS((tl.latitude - $1)/2)),2) +
                       COS(RADIANS($1))*COS(RADIANS(tl.latitude))*
                       POWER(SIN(RADIANS((tl.longitude - $2)/2)),2)
                   )) +
                   erm * 2 * ASIN(SQRT(
                       POWER(SIN(RADIANS((tl.latitude - $3)/2)),2) +
                       COS(RADIANS($3))*COS(RADIANS(tl.latitude))*
                       POWER(SIN(RADIANS((tl.longitude - $4)/2)),2)
                   ))
               ) ASC
           ) as hub_rank
    FROM start_lounges sl
    CROSS JOIN drop_lounges dl
    JOIN lounges tl ON tl.id <> sl.id AND tl.id <> dl.id
    CROSS JOIN haversine
    JOIN lounge_routes lr1  ON lr1.lounge_id  = sl.id
    JOIN lounge_routes lrt1 ON lrt1.lounge_id = tl.id AND lrt1.master_route_id = lr1.master_route_id
    JOIN master_route_stops mrs1_s ON mrs1_s.id = lr1.stop_before_id
    JOIN master_route_stops mrs1_t ON mrs1_t.id = lrt1.stop_before_id
    JOIN lounge_routes lrt2 ON lrt2.lounge_id = tl.id
    JOIN lounge_routes lr2  ON lr2.lounge_id  = dl.id AND lr2.master_route_id = lrt2.master_route_id
    JOIN master_route_stops mrs2_t ON mrs2_t.id = lrt2.stop_before_id
    JOIN master_route_stops mrs2_d ON mrs2_d.id = lr2.stop_before_id
    JOIN master_routes mr1 ON mr1.id = lr1.master_route_id AND mr1.is_active = true
    JOIN master_routes mr2 ON mr2.id = lr2.master_route_id AND mr2.is_active = true
    WHERE mrs1_s.stop_order < mrs1_t.stop_order
      AND mrs2_t.stop_order < mrs2_d.stop_order
),

transit_chains AS (
    SELECT sl_id, sl_name, sl_dist, dl_id, dl_name, dl_dist, tl_id, tl_name, r1_id, r2_id, b1_id, d1_id, b2_id, d2_id
    FROM transit_chains_base
    WHERE hub_rank = 1
    ORDER BY sl_dist ASC, dl_dist ASC, hub_weight ASC
    LIMIT 20
),

direct_results AS (
    SELECT 'direct'::text AS route_type,
           dp.sl_name AS start_lounge_name, dp.dl_name AS drop_lounge_name,
           NULL::text AS transit_lounge_name, NULL::text AS transit_lounge_id,
           dp.sl_dist AS start_dist_m, dp.dl_dist AS drop_dist_m,
           dp.route_name AS r1_route_name, dp.route_number AS r1_route_number,
           dp.route_id::text AS r1_master_id,
           dp.main_route_origin AS r1_main_origin, dp.main_route_destination AS r1_main_destination,
           bs.stop_name AS l1_boarding, ds.stop_name AS l1_dropping,
           s1.trip_id, s1.departure_time AS l1_dep, s1.estimated_arrival AS l1_arr,
           s1.duration_minutes AS l1_duration, s1.total_seats AS l1_total_seats,
           s1.fare AS l1_fare, s1.bus_type AS l1_bus_type,
           s1.is_bookable AS l1_is_bookable,
           s1.has_wifi AS l1_has_wifi, s1.has_ac AS l1_has_ac,
           s1.has_charging_ports AS l1_has_charging_ports,
           s1.has_entertainment AS l1_has_entertainment,
           s1.has_refreshments AS l1_has_refreshments,
           s1.bus_owner_route_id AS l1_bus_owner_route_id,
           s1.trip_master_route_id AS l1_trip_master_id,
           NULL::text AS r2_route_name, NULL::text AS r2_route_number,
           NULL::text AS r2_main_origin, NULL::text AS r2_main_destination,
           NULL::text AS l2_boarding, NULL::text AS l2_dropping,
           NULL::text AS l2_trip_id, NULL::timestamptz AS l2_dep,
           NULL::timestamptz AS l2_arr, 0::float AS l2_fare,
           NULL::text AS l2_bus_type, false AS l2_is_bookable
    FROM direct_pairs dp
    JOIN master_route_stops bs ON bs.id = dp.boarding_stop_id
    JOIN master_route_stops ds ON ds.id = dp.dropping_stop_id
    JOIN LATERAL (
        SELECT st.id::text AS trip_id, st.departure_datetime AS departure_time,
               st.departure_datetime + (COALESCE(st.estimated_duration_minutes,0)*INTERVAL'1 minute') AS estimated_arrival,
               COALESCE(st.estimated_duration_minutes,0) AS duration_minutes,
               COALESCE(bslt.total_seats,0) AS total_seats,
               COALESCE(rp.approved_fare,st.base_fare,0) AS fare,
               COALESCE(b.bus_type,'Normal') AS bus_type, st.is_bookable,
               COALESCE(b.has_wifi,false) AS has_wifi, COALESCE(b.has_ac,false) AS has_ac,
               COALESCE(b.has_charging_ports,false) AS has_charging_ports,
               COALESCE(b.has_entertainment,false) AS has_entertainment,
               COALESCE(b.has_refreshments,false) AS has_refreshments,
               bor.id::text AS bus_owner_route_id,
               COALESCE(bor.master_route_id,rp.master_route_id)::text AS trip_master_route_id
        FROM scheduled_trips st
        LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
        LEFT JOIN route_permits rp     ON rp.id  = st.permit_id
        LEFT JOIN buses b              ON b.license_plate = rp.bus_registration_number
        LEFT JOIN bus_seat_layout_templates bslt ON bslt.id = b.seat_layout_id
        WHERE COALESCE(bor.master_route_id,rp.master_route_id) = dp.route_id
          AND st.is_bookable = true AND st.status IN ('scheduled','confirmed')
          AND st.departure_datetime > ($6 AT TIME ZONE 'Asia/Colombo')
        ORDER BY st.departure_datetime ASC
    ) s1 ON true
),

transit_results AS (
    SELECT 'transit'::text AS route_type,
           tc.sl_name AS start_lounge_name, tc.dl_name AS drop_lounge_name,
           tc.tl_name AS transit_lounge_name, tc.tl_id::text AS transit_lounge_id,
           tc.sl_dist AS start_dist_m, tc.dl_dist AS drop_dist_m,
           mr1.route_name AS r1_route_name, mr1.route_number AS r1_route_number,
           tc.r1_id::text AS r1_master_id,
           mr1.origin_city AS r1_main_origin, mr1.destination_city AS r1_main_destination,
           b1s.stop_name AS l1_boarding, d1s.stop_name AS l1_dropping,
           l1.trip_id, l1.departure_time AS l1_dep, l1.estimated_arrival AS l1_arr,
           l1.duration_minutes AS l1_duration, l1.total_seats AS l1_total_seats,
           l1.fare AS l1_fare, l1.bus_type AS l1_bus_type,
           l1.is_bookable AS l1_is_bookable,
           l1.has_wifi AS l1_has_wifi, l1.has_ac AS l1_has_ac,
           l1.has_charging_ports AS l1_has_charging_ports,
           l1.has_entertainment AS l1_has_entertainment,
           l1.has_refreshments AS l1_has_refreshments,
           NULL::text AS l1_bus_owner_route_id, tc.r1_id::text AS l1_trip_master_id,
           mr2.route_name AS r2_route_name, mr2.route_number AS r2_route_number,
           mr2.origin_city AS r2_main_origin, mr2.destination_city AS r2_main_destination,
           b2s.stop_name AS l2_boarding, d2s.stop_name AS l2_dropping,
           l2.trip_id AS l2_trip_id, l2.departure_time AS l2_dep,
           l2.estimated_arrival AS l2_arr, l2.fare AS l2_fare,
           l2.bus_type AS l2_bus_type, l2.is_bookable AS l2_is_bookable
    FROM transit_chains tc
    JOIN master_routes mr1 ON mr1.id = tc.r1_id
    JOIN master_routes mr2 ON mr2.id = tc.r2_id
    JOIN master_route_stops b1s ON b1s.id = tc.b1_id
    JOIN master_route_stops d1s ON d1s.id = tc.d1_id
    JOIN master_route_stops b2s ON b2s.id = tc.b2_id
    JOIN master_route_stops d2s ON d2s.id = tc.d2_id
    JOIN LATERAL (
        SELECT st.id::text AS trip_id, st.departure_datetime AS departure_time,
               st.departure_datetime + (COALESCE(st.estimated_duration_minutes,0)*INTERVAL'1 minute') AS estimated_arrival,
               COALESCE(st.estimated_duration_minutes,0) AS duration_minutes,
               COALESCE(bslt.total_seats,0) AS total_seats,
               COALESCE(rp.approved_fare,st.base_fare,0) AS fare,
               COALESCE(b.bus_type,'Normal') AS bus_type, st.is_bookable,
               COALESCE(b.has_wifi,false) AS has_wifi, COALESCE(b.has_ac,false) AS has_ac,
               COALESCE(b.has_charging_ports,false) AS has_charging_ports,
               COALESCE(b.has_entertainment,false) AS has_entertainment,
               COALESCE(b.has_refreshments,false) AS has_refreshments
        FROM scheduled_trips st
        LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
        LEFT JOIN route_permits rp     ON rp.id  = st.permit_id
        LEFT JOIN buses b              ON b.license_plate = rp.bus_registration_number
        LEFT JOIN bus_seat_layout_templates bslt ON bslt.id = b.seat_layout_id
        WHERE COALESCE(bor.master_route_id,rp.master_route_id) = tc.r1_id
          AND st.is_bookable = true AND st.status IN ('scheduled','confirmed')
          AND st.departure_datetime > ($6 AT TIME ZONE 'Asia/Colombo')
        ORDER BY st.departure_datetime ASC
    ) l1 ON true
    JOIN LATERAL (
        SELECT st.id::text AS trip_id, st.departure_datetime AS departure_time,
               st.departure_datetime + (COALESCE(st.estimated_duration_minutes,0)*INTERVAL'1 minute') AS estimated_arrival,
               COALESCE(rp.approved_fare,st.base_fare,0) AS fare,
               COALESCE(b.bus_type,'Normal') AS bus_type, st.is_bookable
        FROM scheduled_trips st
        LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
        LEFT JOIN route_permits rp     ON rp.id  = st.permit_id
        LEFT JOIN buses b              ON b.license_plate = rp.bus_registration_number
        WHERE COALESCE(bor.master_route_id,rp.master_route_id) = tc.r2_id
          AND st.is_bookable = true AND st.status IN ('scheduled','confirmed')
          AND st.departure_datetime >= l1.estimated_arrival + INTERVAL '15 minutes'
        ORDER BY st.departure_datetime ASC
    ) l2 ON true
)

SELECT * FROM (
    SELECT * FROM direct_results
    UNION ALL
    SELECT * FROM transit_results
) combined
ORDER BY
    CASE route_type WHEN 'direct' THEN 0 ELSE 1 END,
    start_dist_m ASC, drop_dist_m ASC, l1_dep ASC
LIMIT $7;
`

	type mmRow struct {
		RouteType          string     `db:"route_type"`
		StartLoungeName    string     `db:"start_lounge_name"`
		DropLoungeName     string     `db:"drop_lounge_name"`
		TransitLoungeName  *string    `db:"transit_lounge_name"`
		TransitLoungeID    *string    `db:"transit_lounge_id"`
		StartDistM         float64    `db:"start_dist_m"`
		DropDistM          float64    `db:"drop_dist_m"`
		R1RouteName        string     `db:"r1_route_name"`
		R1RouteNumber      *string    `db:"r1_route_number"`
		R1MasterID         string     `db:"r1_master_id"`
		L1Boarding         string     `db:"l1_boarding"`
		L1Dropping         string     `db:"l1_dropping"`
		TripID             string     `db:"trip_id"`
		L1Dep              time.Time  `db:"l1_dep"`
		L1Arr              time.Time  `db:"l1_arr"`
		L1Duration         int        `db:"l1_duration"`
		L1TotalSeats       int        `db:"l1_total_seats"`
		L1Fare             float64    `db:"l1_fare"`
		L1BusType          string     `db:"l1_bus_type"`
		L1IsBookable       bool       `db:"l1_is_bookable"`
		L1HasWiFi          bool       `db:"l1_has_wifi"`
		L1HasAC            bool       `db:"l1_has_ac"`
		L1HasChargingPorts bool       `db:"l1_has_charging_ports"`
		L1HasEntertainment bool       `db:"l1_has_entertainment"`
		L1HasRefreshments  bool       `db:"l1_has_refreshments"`
		L1BusOwnerRouteID  *string    `db:"l1_bus_owner_route_id"`
		L1TripMasterID     string     `db:"l1_trip_master_id"`
		R1MainOrigin       *string    `db:"r1_main_origin"`
		R1MainDestination  *string    `db:"r1_main_destination"`
		R2RouteName        *string    `db:"r2_route_name"`
		R2RouteNumber      *string    `db:"r2_route_number"`
		R2MainOrigin       *string    `db:"r2_main_origin"`
		R2MainDestination  *string    `db:"r2_main_destination"`
		L2Boarding         *string    `db:"l2_boarding"`
		L2Dropping         *string    `db:"l2_dropping"`
		L2TripID           *string    `db:"l2_trip_id"`
		L2Dep              *time.Time `db:"l2_dep"`
		L2Arr              *time.Time `db:"l2_arr"`
		L2Fare             float64    `db:"l2_fare"`
		L2BusType          *string    `db:"l2_bus_type"`
		L2IsBookable       bool       `db:"l2_is_bookable"`
	}

	var rows []mmRow
	if err := r.db.Select(&rows, mmQuery,
		fromLat, fromLng, toLat, toLng, radiusMeters, afterTime, limit,
	); err != nil {
		return nil, fmt.Errorf("FindMultiModalRoutes: %w", err)
	}

	results := make([]models.TripResult, 0, len(rows))
	for _, row := range rows {
		l1ID, _ := uuid.Parse(row.TripID)
		startL := row.StartLoungeName
		dropL := row.DropLoungeName

		if row.RouteType == "direct" {
			results = append(results, models.TripResult{
				TripID:           l1ID,
				RouteName:        row.R1RouteName,
				RouteNumber:      row.R1RouteNumber,
				BusType:          row.L1BusType,
				DepartureTime:    row.L1Dep,
				EstimatedArrival: row.L1Arr,
				DurationMinutes:  row.L1Duration,
				TotalSeats:       row.L1TotalSeats,
				Fare:             row.L1Fare,
				BoardingPoint:    row.L1Boarding,
				DroppingPoint:    row.L1Dropping,
				FromLounge:       &startL,
				ToLounge:         &dropL,
				FromLoungeDistKm: row.StartDistM / 1000.0,
				ToLoungeDistKm:   row.DropDistM / 1000.0,
				BusFeatures: models.BusFeatures{
					HasWiFi:          row.L1HasWiFi,
					HasAC:            row.L1HasAC,
					HasChargingPorts: row.L1HasChargingPorts,
					HasEntertainment: row.L1HasEntertainment,
					HasRefreshments:  row.L1HasRefreshments,
				},
				IsBookable:           row.L1IsBookable,
				BusOwnerRouteID:      row.L1BusOwnerRouteID,
				MasterRouteID:        &row.L1TripMasterID,
				MainRouteOrigin:      row.R1MainOrigin,
				MainRouteDestination: row.R1MainDestination,
				IsTransit:            false,
			})
		} else {
			// Build transit trip with two legs
			l2Dep, l2Arr := time.Time{}, time.Time{}
			if row.L2Dep != nil {
				l2Dep = *row.L2Dep
			}
			if row.L2Arr != nil {
				l2Arr = *row.L2Arr
			}
			l2Boarding, l2Dropping := "", ""
			if row.L2Boarding != nil {
				l2Boarding = *row.L2Boarding
			}
			if row.L2Dropping != nil {
				l2Dropping = *row.L2Dropping
			}
			r2Name := ""
			if row.R2RouteName != nil {
				r2Name = *row.R2RouteName
			}
			transitHub := ""
			if row.TransitLoungeName != nil {
				transitHub = *row.TransitLoungeName
			}
			var transitHubID *uuid.UUID
			if row.TransitLoungeID != nil {
				if id, err := uuid.Parse(*row.TransitLoungeID); err == nil {
					transitHubID = &id
				}
			}

			l2ID := uuid.UUID{}
			if row.L2TripID != nil {
				l2ID, _ = uuid.Parse(*row.L2TripID)
			}

			leg1 := models.TripResult{
				TripID: l1ID, RouteName: row.R1RouteName, RouteNumber: row.R1RouteNumber,
				BusType: row.L1BusType, DepartureTime: row.L1Dep, EstimatedArrival: row.L1Arr,
				DurationMinutes: row.L1Duration, TotalSeats: row.L1TotalSeats,
				Fare: row.L1Fare, BoardingPoint: row.L1Boarding, DroppingPoint: row.L1Dropping,
				IsBookable: row.L1IsBookable,
				BusFeatures: models.BusFeatures{
					HasWiFi: row.L1HasWiFi, HasAC: row.L1HasAC,
					HasChargingPorts: row.L1HasChargingPorts,
					HasEntertainment: row.L1HasEntertainment,
					HasRefreshments:  row.L1HasRefreshments,
				},
				MainRouteOrigin:      row.R1MainOrigin,
				MainRouteDestination: row.R1MainDestination,
			}
			leg2BusType := "Normal"
			if row.L2BusType != nil {
				leg2BusType = *row.L2BusType
			}
			leg2 := models.TripResult{
				TripID: l2ID, RouteName: r2Name, RouteNumber: row.R2RouteNumber,
				BusType: leg2BusType, DepartureTime: l2Dep, EstimatedArrival: l2Arr,
				Fare: row.L2Fare, BoardingPoint: l2Boarding, DroppingPoint: l2Dropping,
				IsBookable:           row.L2IsBookable,
				MainRouteOrigin:      row.R2MainOrigin,
				MainRouteDestination: row.R2MainDestination,
			}

			results = append(results, models.TripResult{
				TripID:               l1ID,
				RouteName:            fmt.Sprintf("%s → %s", row.R1RouteName, r2Name),
				BusType:              row.L1BusType,
				DepartureTime:        row.L1Dep,
				EstimatedArrival:     l2Arr,
				DurationMinutes:      int(l2Arr.Sub(row.L1Dep).Minutes()),
				TotalSeats:           row.L1TotalSeats,
				Fare:                 row.L1Fare + row.L2Fare,
				BoardingPoint:        row.L1Boarding,
				DroppingPoint:        l2Dropping,
				FromLounge:           &startL,
				ToLounge:             &dropL,
				FromLoungeDistKm:     row.StartDistM / 1000.0,
				ToLoungeDistKm:       row.DropDistM / 1000.0,
				IsBookable:           row.L1IsBookable && row.L2IsBookable,
				IsTransit:            true,
				TransitPoint:         transitHub,
				TransitPointID:       transitHubID,
				Leg1:                 &leg1,
				Leg2:                 &leg2,
				MasterRouteID:        &row.R1MasterID,
				MainRouteOrigin:      row.R1MainOrigin,
				MainRouteDestination: row.R2MainDestination,
			})
		}
	}

	return results, nil
}

// FindLoungeOriginRoutes finds trips that start at a lounge near origin coordinates
// but end at a regular stop matching the destination name.
func (r *SearchRepository) FindLoungeOriginRoutes(
	fromLat, fromLng float64,
	toName string,
	radiusMeters float64,
	afterTime time.Time,
	limit int,
) ([]models.TripResult, error) {

	if limit <= 0 {
		limit = 20
	}

	const loungeOriginQuery = `
WITH
-- 1. Candidate lounges near the ORIGIN
start_lounges AS (
    SELECT
        l.id           AS lounge_id,
        l.lounge_name,
        6371000 * 2 * ASIN(SQRT(
            POWER(SIN(RADIANS((l.latitude  - $1) / 2)), 2) +
            COS(RADIANS($1)) * COS(RADIANS(l.latitude)) *
            POWER(SIN(RADIANS((l.longitude - $2) / 2)), 2)
        )) AS dist_m
    FROM lounges l
    WHERE l.latitude IS NOT NULL AND l.longitude IS NOT NULL
      AND 6371000 * 2 * ASIN(SQRT(
              POWER(SIN(RADIANS((l.latitude  - $1) / 2)), 2) +
              COS(RADIANS($1)) * COS(RADIANS(l.latitude)) *
              POWER(SIN(RADIANS((l.longitude - $2) / 2)), 2)
          )) <= $3
      AND l.status = 'approved' AND l.is_operational = true
    ORDER BY dist_m ASC
),

-- 2. Destination stops matching toName
drop_stops AS (
    SELECT 
        mrs.id as stop_id,
        mrs.stop_name,
        mrs.master_route_id,
        mrs.stop_order
    FROM master_route_stops mrs
    WHERE LOWER(mrs.stop_name) LIKE LOWER('%' || $4 || '%')
),

-- 3. Routes connecting start lounges to destination stops
matched_routes AS (
    SELECT
        sl.lounge_id          AS start_lounge_id,
        sl.lounge_name        AS start_lounge_name,
        sl.dist_m             AS start_dist_m,
        ds.stop_id            AS drop_stop_id,
        ds.stop_name          AS drop_stop_name,
        lr_s.master_route_id,
        mr.route_name,
        mr.route_number,
        lr_s.stop_before_id   AS boarding_stop_id
    FROM start_lounges sl
    JOIN lounge_routes lr_s ON lr_s.lounge_id = sl.lounge_id
    JOIN drop_stops ds ON ds.master_route_id = lr_s.master_route_id
    JOIN master_route_stops mrs_s ON mrs_s.id = lr_s.stop_before_id
    JOIN master_routes mr ON mr.id = lr_s.master_route_id AND mr.is_active = true
    WHERE mrs_s.stop_order < ds.stop_order
    ORDER BY sl.dist_m ASC
)

-- 4. Get scheduled trips
SELECT
    mr_data.start_lounge_name,
    NULL                      AS drop_lounge_name,
    mr_data.route_name,
    mr_data.route_number,
    mr_data.start_dist_m,
    0.0                       AS drop_dist_m,
    mr_data.master_route_id::text,
    boarding_stop.stop_name   AS boarding_point,
    mr_data.drop_stop_name    AS dropping_point,
    sched.trip_id,
    sched.departure_time,
    sched.estimated_arrival,
    sched.duration_minutes,
    sched.total_seats,
    sched.fare,
    sched.bus_type,
    sched.is_bookable,
    sched.has_wifi,
    sched.has_ac,
    sched.has_charging_ports,
    sched.has_entertainment,
    sched.has_refreshments,
    sched.bus_owner_route_id,
    sched.trip_master_route_id
FROM matched_routes mr_data
JOIN master_route_stops boarding_stop ON boarding_stop.id = mr_data.boarding_stop_id
JOIN LATERAL (
    SELECT
        st.id::text                                                             AS trip_id,
        st.departure_datetime                                                   AS departure_time,
        st.departure_datetime +
            (COALESCE(st.estimated_duration_minutes,0) * INTERVAL '1 minute')  AS estimated_arrival,
        COALESCE(st.estimated_duration_minutes, 0)                              AS duration_minutes,
        COALESCE(bslt.total_seats, 0)                                           AS total_seats,
        COALESCE(rp.approved_fare, st.base_fare, 0)                             AS fare,
        COALESCE(b.bus_type, 'Normal')                                          AS bus_type,
        st.is_bookable,
        COALESCE(b.has_wifi,           false) AS has_wifi,
        COALESCE(b.has_ac,             false) AS has_ac,
        COALESCE(b.has_charging_ports, false) AS has_charging_ports,
        COALESCE(b.has_entertainment,  false) AS has_entertainment,
        COALESCE(b.has_refreshments,   false) AS has_refreshments,
        bor.id::text                                                             AS bus_owner_route_id,
        COALESCE(bor.master_route_id, rp.master_route_id)::text                AS trip_master_route_id
    FROM scheduled_trips st
    LEFT JOIN bus_owner_routes bor          ON bor.id = st.bus_owner_route_id
    LEFT JOIN route_permits rp              ON rp.id  = st.permit_id
    LEFT JOIN buses b                       ON b.license_plate = rp.bus_registration_number
    LEFT JOIN bus_seat_layout_templates bslt ON bslt.id = b.seat_layout_id
    WHERE COALESCE(bor.master_route_id, rp.master_route_id) = mr_data.master_route_id
      AND st.is_bookable = true
      AND st.status IN ('scheduled', 'confirmed')
      AND st.departure_datetime > $5
    ORDER BY st.departure_datetime ASC
    LIMIT 3
) sched ON true
ORDER BY mr_data.start_dist_m ASC, sched.departure_time ASC
LIMIT $6
`

	type loungeRow struct {
		StartLoungeName   string    `db:"start_lounge_name"`
		DropLoungeName    *string   `db:"drop_lounge_name"`
		RouteName         string    `db:"route_name"`
		RouteNumber       *string   `db:"route_number"`
		StartDistM        float64   `db:"start_dist_m"`
		DropDistM         float64   `db:"drop_dist_m"`
		MasterRouteID     string    `db:"master_route_id"`
		BoardingPoint     string    `db:"boarding_point"`
		DroppingPoint     string    `db:"dropping_point"`
		TripID            string    `db:"trip_id"`
		DepartureTime     time.Time `db:"departure_time"`
		EstimatedArrival  time.Time `db:"estimated_arrival"`
		DurationMinutes   int       `db:"duration_minutes"`
		TotalSeats        int       `db:"total_seats"`
		Fare              float64   `db:"fare"`
		BusType           string    `db:"bus_type"`
		IsBookable        bool      `db:"is_bookable"`
		HasWiFi           bool      `db:"has_wifi"`
		HasAC             bool      `db:"has_ac"`
		HasChargingPorts  bool      `db:"has_charging_ports"`
		HasEntertainment  bool      `db:"has_entertainment"`
		HasRefreshments   bool      `db:"has_refreshments"`
		BusOwnerRouteID   *string   `db:"bus_owner_route_id"`
		TripMasterRouteID *string   `db:"trip_master_route_id"`
	}

	var rows []loungeRow
	if err := r.db.Select(&rows, loungeOriginQuery,
		fromLat, fromLng,
		radiusMeters,
		toName,
		afterTime,
		limit,
	); err != nil {
		return nil, fmt.Errorf("FindLoungeOriginRoutes: %w", err)
	}

	trips := make([]models.TripResult, 0, len(rows))
	for _, row := range rows {
		tripID, _ := uuid.Parse(row.TripID)
		startLounge := row.StartLoungeName
		trips = append(trips, models.TripResult{
			TripID:           tripID,
			RouteName:        row.RouteName,
			RouteNumber:      row.RouteNumber,
			BusType:          row.BusType,
			DepartureTime:    row.DepartureTime,
			EstimatedArrival: row.EstimatedArrival,
			DurationMinutes:  row.DurationMinutes,
			TotalSeats:       row.TotalSeats,
			Fare:             row.Fare,
			BoardingPoint:    row.BoardingPoint,
			DroppingPoint:    row.DroppingPoint,
			FromLounge:       &startLounge,
			ToLounge:         nil,
			FromLoungeDistKm: row.StartDistM / 1000.0,
			BusFeatures: models.BusFeatures{
				HasWiFi:          row.HasWiFi,
				HasAC:            row.HasAC,
				HasChargingPorts: row.HasChargingPorts,
				HasEntertainment: row.HasEntertainment,
				HasRefreshments:  row.HasRefreshments,
			},
			IsBookable:      row.IsBookable,
			BusOwnerRouteID: row.BusOwnerRouteID,
			MasterRouteID:   row.TripMasterRouteID,
		})
	}

	return trips, nil
}

// HasLoungesWithinRadius checks if there are any lounges within the given radius of a point
func (r *SearchRepository) HasLoungesWithinRadius(lat, lng, radiusMeters float64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM lounges
			WHERE status = 'approved' AND latitude IS NOT NULL AND longitude IS NOT NULL
			  AND 6371000 * 2 * ASIN(SQRT(
				  POWER(SIN(RADIANS((latitude  - $1) / 2)), 2) +
				  COS(RADIANS($1)) * COS(RADIANS(latitude)) *
				  POWER(SIN(RADIANS((longitude - $2) / 2)), 2)
			  )) <= $3
		)
	`
	var exists bool
	err := r.db.Get(&exists, query, lat, lng, radiusMeters)
	return exists, err
}

// FindLoungeTransitRoutes implements the Dynamic One-Transit Route Discovery algorithm.
// It finds paths between Start Lounge -> Transit Lounge -> Drop Lounge.
func (r *SearchRepository) FindLoungeTransitRoutes(
	fromLat, fromLng float64,
	toLat, toLng float64,
	radiusMeters float64,
	afterTime time.Time,
	limit int,
) ([]models.TripResult, error) {

	if limit <= 0 {
		limit = 10
	}

	const transitQuery = `
WITH 
-- 1. Nearest Single Start Lounge (approved, locked to 1)
start_lounges AS (
    SELECT id, lounge_name, latitude, longitude,
           6371000 * 2 * ASIN(SQRT(POWER(SIN(RADIANS((latitude - $1)/2)),2) + COS(RADIANS($1))*COS(RADIANS(latitude))*POWER(SIN(RADIANS((longitude - $2)/2)),2))) as dist_m
    FROM lounges 
    WHERE status = 'approved' AND latitude IS NOT NULL AND longitude IS NOT NULL
      AND 6371000 * 2 * ASIN(SQRT(POWER(SIN(RADIANS((latitude - $1)/2)),2) + COS(RADIANS($1))*COS(RADIANS(latitude))*POWER(SIN(RADIANS((longitude - $2)/2)),2))) <= $5
    ORDER BY dist_m ASC
    LIMIT 1
),
-- 2. Nearest Single Drop Lounge (approved, locked to 1)
drop_lounges AS (
    SELECT id, lounge_name, latitude, longitude,
           6371000 * 2 * ASIN(SQRT(POWER(SIN(RADIANS((latitude - $3)/2)),2) + COS(RADIANS($3))*COS(RADIANS(latitude))*POWER(SIN(RADIANS((longitude - $4)/2)),2))) as dist_m
    FROM lounges 
    WHERE status = 'approved' AND latitude IS NOT NULL AND longitude IS NOT NULL
      AND 6371000 * 2 * ASIN(SQRT(POWER(SIN(RADIANS((latitude - $3)/2)),2) + COS(RADIANS($3))*COS(RADIANS(latitude))*POWER(SIN(RADIANS((longitude - $4)/2)),2))) <= $5
    ORDER BY dist_m ASC
    LIMIT 1
),
-- 3. Potential Transit Lounges (approved only, exclude start/drop)
transit_lounges AS (
    SELECT id, lounge_name FROM lounges WHERE status = 'approved'
),
-- 4. One-Transit Chain Discovery: for the fixed start+drop pair, find the nearest transit hub
matched_chains AS (
    SELECT 
        sl.id as sl_id, sl.lounge_name as sl_name, sl.dist_m as sl_dist,
        tl.id as tl_id, tl.lounge_name as tl_name,
        dl.id as dl_id, dl.lounge_name as dl_name, dl.dist_m as dl_dist,
        lr1.master_route_id as r1_id, lr2.master_route_id as r2_id,
        lr1.stop_before_id as b1_id, lrt1.stop_before_id as d1_id,
        lrt2.stop_before_id as b2_id, lr2.stop_before_id as d2_id,
        -- Calculate transit hub deviation from direct path midpoint for nearest hub selection
        ABS(mrs1_t.stop_order - mrs1_s.stop_order) + ABS(mrs2_d.stop_order - mrs2_t.stop_order) AS total_stop_hops
    FROM start_lounges sl
    CROSS JOIN drop_lounges dl
    JOIN transit_lounges tl ON tl.id NOT IN (sl.id, dl.id)
    -- Leg 1: sl -> tl
    JOIN lounge_routes lr1 ON lr1.lounge_id = sl.id
    JOIN lounge_routes lrt1 ON lrt1.lounge_id = tl.id AND lrt1.master_route_id = lr1.master_route_id
    JOIN master_route_stops mrs1_s ON mrs1_s.id = lr1.stop_before_id
    JOIN master_route_stops mrs1_t ON mrs1_t.id = lrt1.stop_before_id
    -- Leg 2: tl -> dl
    JOIN lounge_routes lrt2 ON lrt2.lounge_id = tl.id
    JOIN lounge_routes lr2 ON lr2.lounge_id = dl.id AND lr2.master_route_id = lrt2.master_route_id
    JOIN master_route_stops mrs2_t ON mrs2_t.id = lrt2.stop_before_id
    JOIN master_route_stops mrs2_d ON mrs2_d.id = lr2.stop_before_id
    WHERE mrs1_s.stop_order < mrs1_t.stop_order
      AND mrs2_t.stop_order < mrs2_d.stop_order
    -- Select the single best (nearest) transit hub by minimizing total stop hops
    ORDER BY total_stop_hops ASC, sl.dist_m ASC, dl.dist_m ASC
    LIMIT 1
)
SELECT 
    mc.sl_name as start_lounge_name, mc.sl_dist as start_dist_m,
    mc.tl_name as transit_lounge_name, mc.tl_id::text as transit_lounge_id,
    mc.dl_name as drop_lounge_name, mc.dl_dist as drop_dist_m,
    -- Leg 1 Details
    l1.trip_id as l1_trip_id, l1.departure_time as l1_dep, l1.estimated_arrival as l1_arr,
    l1.route_name as l1_route_name, l1.route_number as l1_route_number, l1.fare as l1_fare,
    l1.bus_type as l1_bus_type, l1.master_route_id as l1_master_id,
    b1.stop_name as l1_boarding, d1.stop_name as l1_dropping,
    -- Leg 2 Details
    l2.trip_id as l2_trip_id, l2.departure_time as l2_dep, l2.estimated_arrival as l2_arr,
    l2.route_name as l2_route_name, l2.route_number as l2_route_number, l2.fare as l2_fare,
    l2.bus_type as l2_bus_type, l2.master_route_id as l2_master_id,
    b2.stop_name as l2_boarding, d2.stop_name as l2_dropping
FROM matched_chains mc
JOIN master_route_stops b1 ON b1.id = mc.b1_id
JOIN master_route_stops d1 ON d1.id = mc.d1_id
JOIN master_route_stops b2 ON b2.id = mc.b2_id
JOIN master_route_stops d2 ON d2.id = mc.d2_id
JOIN LATERAL (
    SELECT st.id::text as trip_id, st.departure_datetime as departure_time,
           st.departure_datetime + (COALESCE(st.estimated_duration_minutes,0) * INTERVAL '1 minute') as estimated_arrival,
           mr.route_name, mr.route_number, COALESCE(st.base_fare, 0) as fare,
           COALESCE(b.bus_type, 'Normal') as bus_type, mr.id::text as master_route_id
    FROM scheduled_trips st
    JOIN master_routes mr ON mr.id = st.master_route_id
    LEFT JOIN buses b ON b.id = st.bus_id
    WHERE st.master_route_id = mc.r1_id AND st.departure_datetime >= $6
    ORDER BY st.departure_datetime ASC LIMIT 1
) l1 ON true
JOIN LATERAL (
    SELECT st.id::text as trip_id, st.departure_datetime as departure_time,
           st.departure_datetime + (COALESCE(st.estimated_duration_minutes,0) * INTERVAL '1 minute') as estimated_arrival,
           mr.route_name, mr.route_number, COALESCE(st.base_fare, 0) as fare,
           COALESCE(b.bus_type, 'Normal') as bus_type, mr.id::text as master_route_id
    FROM scheduled_trips st
    JOIN master_routes mr ON mr.id = st.master_route_id
    LEFT JOIN buses b ON b.id = st.bus_id
    WHERE st.master_route_id = mc.r2_id 
      AND st.departure_datetime >= l1.estimated_arrival + INTERVAL '15 minutes'
    ORDER BY st.departure_datetime ASC LIMIT 1
) l2 ON true
LIMIT $7;
`

	type transitRow struct {
		StartLoungeName   string    `db:"start_lounge_name"`
		StartDistM        float64   `db:"start_dist_m"`
		TransitLoungeName string    `db:"transit_lounge_name"`
		TransitLoungeID   string    `db:"transit_lounge_id"`
		DropLoungeName    string    `db:"drop_lounge_name"`
		DropDistM         float64   `db:"drop_dist_m"`
		L1TripID          string    `db:"l1_trip_id"`
		L1Dep             time.Time `db:"l1_dep"`
		L1Arr             time.Time `db:"l1_arr"`
		L1RouteName       string    `db:"l1_route_name"`
		L1RouteNumber     *string   `db:"l1_route_number"`
		L1Fare            float64   `db:"l1_fare"`
		L1BusType         string    `db:"l1_bus_type"`
		L1MasterID        string    `db:"l1_master_id"`
		L1Boarding        string    `db:"l1_boarding"`
		L1Dropping        string    `db:"l1_dropping"`
		L2TripID          string    `db:"l2_trip_id"`
		L2Dep             time.Time `db:"l2_dep"`
		L2Arr             time.Time `db:"l2_arr"`
		L2RouteName       string    `db:"l2_route_name"`
		L2RouteNumber     *string   `db:"l2_route_number"`
		L2Fare            float64   `db:"l2_fare"`
		L2BusType         string    `db:"l2_bus_type"`
		L2MasterID        string    `db:"l2_master_id"`
		L2Boarding        string    `db:"l2_boarding"`
		L2Dropping        string    `db:"l2_dropping"`
	}

	var rows []transitRow
	err := r.db.Select(&rows, transitQuery, fromLat, fromLng, toLat, toLng, radiusMeters, afterTime, limit)
	if err != nil {
		return nil, err
	}

	results := make([]models.TripResult, 0, len(rows))
	for _, row := range rows {
		l1ID, _ := uuid.Parse(row.L1TripID)
		l2ID, _ := uuid.Parse(row.L2TripID)
		tLoungeID, _ := uuid.Parse(row.TransitLoungeID)

		leg1 := models.TripResult{
			TripID:           l1ID,
			RouteName:        row.L1RouteName,
			RouteNumber:      row.L1RouteNumber,
			DepartureTime:    row.L1Dep,
			EstimatedArrival: row.L1Arr,
			Fare:             row.L1Fare,
			BusType:          row.L1BusType,
			BoardingPoint:    row.L1Boarding,
			DroppingPoint:    row.L1Dropping,
			IsBookable:       true, // Basic trips from lateral are bookable
		}

		leg2 := models.TripResult{
			TripID:           l2ID,
			RouteName:        row.L2RouteName,
			RouteNumber:      row.L2RouteNumber,
			DepartureTime:    row.L2Dep,
			EstimatedArrival: row.L2Arr,
			Fare:             row.L2Fare,
			BusType:          row.L2BusType,
			BoardingPoint:    row.L2Boarding,
			DroppingPoint:    row.L2Dropping,
			IsBookable:       true,
		}

		duration := int(row.L2Arr.Sub(row.L1Dep).Minutes())

		results = append(results, models.TripResult{
			TripID:           l1ID, // Use Leg 1 ID as the primary ID for the combined trip
			IsTransit:        true,
			TransitPointID:   &tLoungeID,
			TransitPoint:     row.TransitLoungeName,
			FromLounge:       &row.StartLoungeName,
			ToLounge:         &row.DropLoungeName,
			FromLoungeDistKm: row.StartDistM / 1000.0,
			ToLoungeDistKm:   row.DropDistM / 1000.0,
			Leg1:             &leg1,
			Leg2:             &leg2,
			Fare:             row.L1Fare + row.L2Fare,
			DepartureTime:    row.L1Dep,
			EstimatedArrival: row.L2Arr,
			DurationMinutes:  duration,
			RouteName:        fmt.Sprintf("%s ➔ %s", row.L1RouteName, row.L2RouteName),
			BusType:          row.L1BusType,
			IsBookable:       true,
			TotalSeats:       40, // Default for now
			BusFeatures: models.BusFeatures{
				HasWiFi: true, // Transit routes usually high quality
				HasAC:   true,
			},
		})
	}

	return results, nil
}
