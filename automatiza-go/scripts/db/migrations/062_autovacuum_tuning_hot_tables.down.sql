-- Reverte o tuning de autovacuum para os defaults do cluster.
ALTER TABLE ml_link_list_items RESET (autovacuum_vacuum_scale_factor, autovacuum_analyze_scale_factor, autovacuum_vacuum_cost_delay);
ALTER TABLE dispatch_logs      RESET (autovacuum_vacuum_scale_factor, autovacuum_analyze_scale_factor, autovacuum_vacuum_cost_delay);
ALTER TABLE webhook_events     RESET (autovacuum_vacuum_scale_factor, autovacuum_analyze_scale_factor, autovacuum_vacuum_cost_delay);
ALTER TABLE automation_runs    RESET (autovacuum_vacuum_scale_factor, autovacuum_analyze_scale_factor, autovacuum_vacuum_cost_delay);
ALTER TABLE action_history     RESET (autovacuum_vacuum_scale_factor, autovacuum_analyze_scale_factor, autovacuum_vacuum_cost_delay);
