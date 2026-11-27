import os
import re

service_file = r'internal\services\booking_orchestrator_service.go'
with open(service_file, 'r', encoding='utf-8') as f:
    text = f.read()

# 1. Update signature
text = text.replace(
    '''func (s *BookingOrchestratorService) AddLoungeToIntent(
\tintentID uuid.UUID,
\tuserID uuid.UUID,
\tpreTripLounge *models.LoungeIntentPayload,
\ttransitLounge *models.LoungeIntentPayload,
\tpostTripLounge *models.LoungeIntentPayload,
)''',
    '''func (s *BookingOrchestratorService) AddLoungeToIntent(
\tintentID uuid.UUID,
\tuserID uuid.UUID,
\tpreTripLounge *models.LoungeIntentPayload,
\ttransitLounge *models.LoungeIntentPayload,
\tpostTripLounge *models.LoungeIntentPayload,
\treturnPreTripLounge *models.LoungeIntentPayload,
\treturnPostTripLounge *models.LoungeIntentPayload,
)'''
)

# 2. Update vars
text = text.replace(
    'var preLoungeFare, transitLoungeFare, postLoungeFare float64',
    'var preLoungeFare, transitLoungeFare, postLoungeFare, returnPreLoungeFare, returnPostLoungeFare float64'
)

# 3. Add block for return lounges before "// 3. Update intent with lounge data"
return_lounges_block = '''
if returnPreTripLounge != nil {
loungeID, _ := uuid.Parse(returnPreTripLounge.LoungeID)
lounge, err := s.loungeRepo.GetLoungeByID(loungeID)
if err == nil && lounge != nil && lounge.Status == "approved" && lounge.IsOperational {
returnPreLoungeFare = returnPreTripLounge.TotalPrice
expiresAt := time.Now().Add(s.config.IntentTTL)
loungeDate := parseLoungeDate(returnPreTripLounge.Date)
checkInTime := returnPreTripLounge.CheckInTime
if checkInTime == "" { checkInTime = "09:00" }
checkOutTime := calculateCheckoutTime(checkInTime, returnPreTripLounge.PricingType)

hold := &models.LoungeCapacityHold{
ID:            uuid.New(),
LoungeID:      loungeID,
IntentID:      intent.ID,
Date:          loungeDate,
TimeSlotStart: checkInTime,
TimeSlotEnd:   checkOutTime,
GuestsCount:   returnPreTripLounge.GuestCount,
HeldUntil:     expiresAt,
Status:        "held",
CreatedAt:     time.Now(),
}
s.intentRepo.CreateLoungeCapacityHold(hold)
}
}

if returnPostTripLounge != nil {
loungeID, _ := uuid.Parse(returnPostTripLounge.LoungeID)
lounge, err := s.loungeRepo.GetLoungeByID(loungeID)
if err == nil && lounge != nil && lounge.Status == "approved" && lounge.IsOperational {
returnPostLoungeFare = returnPostTripLounge.TotalPrice
expiresAt := time.Now().Add(s.config.IntentTTL)
loungeDate := parseLoungeDate(returnPostTripLounge.Date)
checkInTime := returnPostTripLounge.CheckInTime
if checkInTime == "" { checkInTime = "09:00" }
checkOutTime := calculateCheckoutTime(checkInTime, returnPostTripLounge.PricingType)

hold := &models.LoungeCapacityHold{
ID:            uuid.New(),
LoungeID:      loungeID,
IntentID:      intent.ID,
Date:          loungeDate,
TimeSlotStart: checkInTime,
TimeSlotEnd:   checkOutTime,
GuestsCount:   returnPostTripLounge.GuestCount,
HeldUntil:     expiresAt,
Status:        "held",
CreatedAt:     time.Now(),
}
s.intentRepo.CreateLoungeCapacityHold(hold)
}
}

// 3. Update intent with lounge data'''

text = text.replace('// 3. Update intent with lounge data', return_lounges_block)

# 4. update newTotal
text = text.replace(
    'newTotal := intent.BusFare + preLoungeFare + transitLoungeFare + postLoungeFare',
    'newTotal := intent.BusFare + intent.ReturnBusFare + preLoungeFare + transitLoungeFare + postLoungeFare + returnPreLoungeFare + returnPostLoungeFare'
)

# 5. update logrus fields
text = text.replace(
    '''"post_lounge_fare":    postLoungeFare,
"new_total":           newTotal,''',
    '''"post_lounge_fare":    postLoungeFare,
"return_pre_lounge":   returnPreTripLounge != nil,
"return_post_lounge":  returnPostTripLounge != nil,
"new_total":           newTotal,'''
)

# 6. update s.intentRepo.AddLoungeToIntent
text = text.replace(
    '''err = s.intentRepo.AddLoungeToIntent(
intent.ID,
preTripLounge,
transitLounge,
postTripLounge,
preLoungeFare,
transitLoungeFare,
postLoungeFare,
newTotal,
newExpiresAt,
)''',
    '''err = s.intentRepo.AddLoungeToIntent(
intent.ID,
preTripLounge,
transitLounge,
postTripLounge,
returnPreTripLounge,
returnPostTripLounge,
preLoungeFare,
transitLoungeFare,
postLoungeFare,
returnPreLoungeFare,
returnPostLoungeFare,
newTotal,
newExpiresAt,
)'''
)

with open(service_file, 'w', encoding='utf-8') as f:
    f.write(text)


repo_file = r'internal\database\booking_intent_repository.go'
with open(repo_file, 'r', encoding='utf-8') as f:
    rtext = f.read()

rtext = rtext.replace(
    '''func (r *BookingIntentRepository) AddLoungeToIntent(
intentID uuid.UUID,
preTripLounge *models.LoungeIntentPayload,
transitLounge *models.LoungeIntentPayload,
postTripLounge *models.LoungeIntentPayload,
preLoungeFare float64,
transitLoungeFare float64,
postLoungeFare float64,
newTotal float64,
newExpiresAt time.Time,
) error {
r.logger.WithFields(logrus.Fields{
"intent_id":           intentID,
"pre_lounge_fare":     preLoungeFare,
"transit_lounge_fare": transitLoungeFare,
"post_lounge_fare":    postLoungeFare,
"new_total":           newTotal,
}).Debug("Updating intent with lounge data")

updates := map[string]interface{}{
"pre_trip_lounge_intent":  preTripLounge,
"transit_lounge_intent":   transitLounge,
"post_trip_lounge_intent": postTripLounge,
"pre_lounge_fare":         preLoungeFare,
"transit_lounge_fare":     transitLoungeFare,
"post_lounge_fare":        postLoungeFare,
"total_amount":            newTotal,
"expires_at":              newExpiresAt,
"updated_at":              time.Now(),
}''',
    '''func (r *BookingIntentRepository) AddLoungeToIntent(
intentID uuid.UUID,
preTripLounge *models.LoungeIntentPayload,
transitLounge *models.LoungeIntentPayload,
postTripLounge *models.LoungeIntentPayload,
returnPreTripLounge *models.LoungeIntentPayload,
returnPostTripLounge *models.LoungeIntentPayload,
preLoungeFare float64,
transitLoungeFare float64,
postLoungeFare float64,
returnPreLoungeFare float64,
returnPostLoungeFare float64,
newTotal float64,
newExpiresAt time.Time,
) error {
r.logger.WithFields(logrus.Fields{
"intent_id":           intentID,
"pre_lounge_fare":     preLoungeFare,
"transit_lounge_fare": transitLoungeFare,
"post_lounge_fare":    postLoungeFare,
"return_pre_fare":     returnPreLoungeFare,
"return_post_fare":    returnPostLoungeFare,
"new_total":           newTotal,
}).Debug("Updating intent with lounge data")

updates := map[string]interface{}{
"pre_trip_lounge_intent":          preTripLounge,
"transit_lounge_intent":           transitLounge,
"post_trip_lounge_intent":         postTripLounge,
"return_pre_trip_lounge_intent":   returnPreTripLounge,
"return_post_trip_lounge_intent":  returnPostTripLounge,
"pre_lounge_fare":                 preLoungeFare,
"transit_lounge_fare":             transitLoungeFare,
"post_lounge_fare":                postLoungeFare,
"return_pre_trip_lounge_fare":     returnPreLoungeFare,
"return_post_trip_lounge_fare":    returnPostLoungeFare,
"total_amount":                    newTotal,
"expires_at":                      newExpiresAt,
"updated_at":                      time.Now(),
}'''
)

with open(repo_file, 'w', encoding='utf-8') as f:
    f.write(rtext)
