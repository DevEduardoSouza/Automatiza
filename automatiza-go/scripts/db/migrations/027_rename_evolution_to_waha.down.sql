-- Revert waha_api_* settings keys back to evolution_api_*.
UPDATE settings SET key = 'evolution_api_key' WHERE key = 'waha_api_key';
UPDATE settings SET key = 'evolution_api_url' WHERE key = 'waha_api_url';
