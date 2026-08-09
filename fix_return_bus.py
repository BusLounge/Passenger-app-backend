import os
import re

service_file = r'internal\services\booking_orchestrator_service.go'
with open(service_file, 'r', encoding='utf-8') as f:
    text = f.read()

text = text.replace(
    'intent.ReturnBusFare',
    'intent.PricingSnapshot.ReturnBusFare'
)

with open(service_file, 'w', encoding='utf-8') as f:
    f.write(text)
