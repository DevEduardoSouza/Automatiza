-- Remove 'send_ml_offer' from jobs.action_type CHECK constraint.
-- Zero rows in production use this action type (verified 2026-04-18); users
-- now manage ML dispatches via LIST mode automations, not one-off jobs.
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS chk_jobs_action_type;
ALTER TABLE jobs ADD CONSTRAINT chk_jobs_action_type CHECK (action_type IN (
    'send_message', 'send_poll',
    'update_description', 'close_groups', 'open_groups',
    'change_group_name', 'change_group_photo'
));
