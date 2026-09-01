package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/pkg/sms"
)

// defaultLowBalanceThreshold is the default threshold in LKR below which users are notified
const defaultLowBalanceThreshold = 500.0

type WalletService struct {
	walletRepo    database.WalletRepository
	passengerRepo *database.PassengerRepository
	smsGateway    sms.SMSGateway
	threshold     float64
	logger        *logrus.Logger
}

func NewWalletService(
	walletRepo database.WalletRepository,
	passengerRepo *database.PassengerRepository,
	smsGateway sms.SMSGateway,
	threshold float64,
	logger *logrus.Logger,
) *WalletService {
	if threshold <= 0 {
		threshold = defaultLowBalanceThreshold
	}
	return &WalletService{
		walletRepo:    walletRepo,
		passengerRepo: passengerRepo,
		smsGateway:    smsGateway,
		threshold:     threshold,
		logger:        logger,
	}
}

// GetWalletData retrieves the wallet information and recent transactions for a user
func (s *WalletService) GetWalletData(userID uuid.UUID) (map[string]interface{}, error) {
	wallet, err := s.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get wallet for user")
		return nil, fmt.Errorf("failed to retrieve wallet data")
	}

	transactions, err := s.walletRepo.GetWalletTransactions(wallet.ID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get wallet transactions")
		transactions = []models.WalletTransaction{}
	}

	data := map[string]interface{}{
		"wallet": map[string]interface{}{
			"id":      wallet.ID,
			"balance": wallet.Balance,
			"status":  wallet.Status,
		},
		"transactions": transactions,
	}

	return data, nil
}

// ConfirmTopUp processes a confirmed top-up reference and credits the digital wallet
func (s *WalletService) ConfirmTopUp(userID uuid.UUID, amount float64, gatewayRef string) error {
	err := s.walletRepo.ConfirmTopUp(userID, amount, gatewayRef)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"user_id":     userID,
			"amount":      amount,
			"gateway_ref": gatewayRef,
		}).Error("Failed to confirm wallet top-up")
		return fmt.Errorf("failed to confirm top-up: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id":     userID,
		"amount":      amount,
		"gateway_ref": gatewayRef,
	}).Info("Wallet top-up successfully confirmed via PayHere")

	// Fire low-balance check asynchronously so it never blocks the response
	go s.checkAndNotifyLowBalance(userID)

	return nil
}

// DeductBalance deducts an amount from the user's wallet
func (s *WalletService) DeductBalance(userID uuid.UUID, amount float64, reference string, description string) error {
	wallet, err := s.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return fmt.Errorf("failed to get wallet: %w", err)
	}

	if wallet.Balance < amount {
		return fmt.Errorf("insufficient wallet balance")
	}

	err = s.walletRepo.DeductBalance(userID, amount, reference)
	if err != nil {
		return fmt.Errorf("failed to deduct balance: %w", err)
	}

	// Fire low-balance check asynchronously after every deduction
	go s.checkAndNotifyLowBalance(userID)

	return nil
}

// checkAndNotifyLowBalance fetches the current balance and sends an SMS if it is below the threshold.
// Runs in a goroutine — must not panic.
func (s *WalletService) checkAndNotifyLowBalance(userID uuid.UUID) {
	balance, err := s.walletRepo.GetWalletBalance(userID)
	if err != nil {
		s.logger.WithError(err).Warn("Low-balance check: failed to fetch balance")
		return
	}

	if balance >= s.threshold {
		return // balance is fine — nothing to do
	}

	if s.passengerRepo == nil || s.smsGateway == nil {
		s.logger.Warn("Low-balance check: passengerRepo or smsGateway not configured, skipping notification")
		return
	}

	phone, err := s.passengerRepo.GetUserPhone(userID)
	if err != nil || phone == "" {
		s.logger.WithError(err).Warn("Low-balance check: failed to get user phone for notification")
		return
	}

	message := fmt.Sprintf(
		"SmartTransit Alert: Your E-Wallet balance is low (LKR %.2f). Top up now to continue booking trips smoothly.",
		balance,
	)

	_, err = s.smsGateway.SendBulkSMS([]string{phone}, message)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"user_id": userID,
			"balance": balance,
			"phone":   phone,
		}).Error("Failed to send low-balance SMS notification")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"balance": balance,
		"phone":   phone,
	}).Info("Low-balance SMS notification sent successfully")
}
