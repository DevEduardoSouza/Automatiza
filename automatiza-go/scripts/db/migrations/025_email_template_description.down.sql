-- Remove description column from email_templates table
ALTER TABLE email_templates DROP COLUMN IF EXISTS description;
