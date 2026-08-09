import os

filepath = r'internal\services\booking_orchestrator_service.go'
with open(filepath, 'r', encoding='utf-8') as f:
    text = f.read()

text = text.replace(
    'var busBookingID, preLoungeBookingID, transitLoungeBookingID, postLoungeBookingID *uuid.UUID',
    'var busBookingID, returnBusBookingID, preLoungeBookingID, transitLoungeBookingID, postLoungeBookingID, returnPreLoungeBookingID, returnPostLoungeBookingID *uuid.UUID'
)

# Fix returnBusBookingID assignment (if returnBusBooking exists)
# Let's see if returnBusBooking is assigned anywhere... We'll find out if it's there.
# If not, let's just make sure the UpdateIntentConfirmed call has the correct arguments.
text = text.replace(
    'if err := s.intentRepo.UpdateIntentConfirmed(intent.ID, busBookingID, preLoungeBookingID, transitLoungeBookingID, postLoungeBookingID); err != nil {',
    'if err := s.intentRepo.UpdateIntentConfirmed(intent.ID, busBookingID, returnBusBookingID, preLoungeBookingID, transitLoungeBookingID, postLoungeBookingID, returnPreLoungeBookingID, returnPostLoungeBookingID); err != nil {'
)

# And now we manually fix the returnPreLoungeBookingID assignment
text = text.replace(
    '''returnPreBooking, err := s.createLoungeBookingFromIntent(intent, intent.PreTripLoungeIntent.ReturnLounge, "return_pre_trip", masterBookingID, busBookingID)
if err != nil {
s.logger.WithError(err).Error("Failed to create return pre-trip lounge booking")
} else {
s.logger.WithField("return_pre_lounge_booking_id", returnPreBooking.ID).Info("Return pre-trip lounge booking inserted successfully")

s.loungeBookingRepo.UpdateLoungeBookingStatus(returnPreBooking.ID, models.LoungeBookingStatusConfirmed)
s.loungeBookingRepo.UpdatePaymentStatus(returnPreBooking.ID, models.LoungePaymentPaid)
}''',
    '''returnPreBooking, err := s.createLoungeBookingFromIntent(intent, intent.PreTripLoungeIntent.ReturnLounge, "return_pre_trip", masterBookingID, busBookingID)
if err != nil {
s.logger.WithError(err).Error("Failed to create return pre-trip lounge booking")
} else {
s.logger.WithField("return_pre_lounge_booking_id", returnPreBooking.ID).Info("Return pre-trip lounge booking inserted successfully")
id := returnPreBooking.ID
returnPreLoungeBookingID = &id
s.loungeBookingRepo.UpdateLoungeBookingStatus(returnPreBooking.ID, models.LoungeBookingStatusConfirmed)
s.loungeBookingRepo.UpdatePaymentStatus(returnPreBooking.ID, models.LoungePaymentPaid)
}'''
)

text = text.replace(
    '''returnPostBooking, err := s.createLoungeBookingFromIntent(intent, intent.PostTripLoungeIntent.ReturnLounge, "return_post_trip", masterBookingID, busBookingID)
if err != nil {
s.logger.WithError(err).Error("Failed to create return post-trip lounge booking")
} else {
s.logger.WithField("return_post_lounge_booking_id", returnPostBooking.ID).Info("Return post-trip lounge booking inserted successfully")

s.loungeBookingRepo.UpdateLoungeBookingStatus(returnPostBooking.ID, models.LoungeBookingStatusConfirmed)
s.loungeBookingRepo.UpdatePaymentStatus(returnPostBooking.ID, models.LoungePaymentPaid)
}''',
    '''returnPostBooking, err := s.createLoungeBookingFromIntent(intent, intent.PostTripLoungeIntent.ReturnLounge, "return_post_trip", masterBookingID, busBookingID)
if err != nil {
s.logger.WithError(err).Error("Failed to create return post-trip lounge booking")
} else {
s.logger.WithField("return_post_lounge_booking_id", returnPostBooking.ID).Info("Return post-trip lounge booking inserted successfully")
id := returnPostBooking.ID
returnPostLoungeBookingID = &id
s.loungeBookingRepo.UpdateLoungeBookingStatus(returnPostBooking.ID, models.LoungeBookingStatusConfirmed)
s.loungeBookingRepo.UpdatePaymentStatus(returnPostBooking.ID, models.LoungePaymentPaid)
}'''
)


with open(filepath, 'w', encoding='utf-8') as f:
    f.write(text)
