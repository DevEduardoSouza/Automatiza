-- Rename evolution_api_* settings keys to waha_api_* for existing databases.
UPDATE settings SET key = 'waha_api_key' WHERE key = 'evolution_api_key';
UPDATE settings SET key = 'waha_api_url' WHERE key = 'evolution_api_url';
