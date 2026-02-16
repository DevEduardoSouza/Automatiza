-- Reset failed_attempts and locked_until for users with placeholder hash (migration from legacy Supabase)
-- These users cannot log in because their password_hash is a non-matching placeholder,
-- so failed_attempts/lockout state is meaningless and should be cleared.
UPDATE user_credentials
SET failed_attempts = 0, locked_until = NULL
WHERE password_hash = '$2a$10$MIGRATED.FROM.LEGACY.SYSTEM.REQUIRES.PASSWORD.RESET.000000'
  AND (failed_attempts > 0 OR locked_until IS NOT NULL);
