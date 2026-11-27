package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

func main() {
	db, err := sqlx.Connect("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres")
	if err != nil {
		fmt.Println("DB err:", err)
		return
	}
	repo := database.NewLoungeBookingRepository(db)
	masterID := uuid.New()
	booking := &models.LoungeBooking{
		UserID:           uuid.MustParse("00000000-0000-0000-0000-000000000000"),
		LoungeID:         uuid.New(),
		MasterBookingID:  &masterID,
		ScheduledArrival: time.Now(),
		NumberOfGuests:   1,
		PricingType:      "1_hour",
		PricePerGuest:    "1000.00",
		BasePrice:        "1000.00",
		PreOrderTotal:    "0.00",
		TotalAmount:      "1000.00",
		LoungeName:       "Test Lounge",
		PrimaryGuestName: "Test Guest", DiscountAmount: "0.00",
	}
	guests := []models.LoungeBookingGuest{
		{GuestName: "Test Guest", IsPrimaryGuest: true},
	}
	_, err = repo.CreateLoungeBooking(booking, guests, nil)
	fmt.Println("Result:", err)
}
