ALTER TABLE jobs DROP CONSTRAINT IF EXISTS chk_jobs_action_type;
ALTER TABLE jobs ADD CONSTRAINT chk_jobs_action_type CHECK (action_type IN (
    'send_message', 'send_poll', 'send_ml_offer',
    'update_description', 'close_groups', 'open_groups',
    'change_group_name', 'change_group_photo'
));
