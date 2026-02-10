-- Add description column to email_templates table
ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS description TEXT;
