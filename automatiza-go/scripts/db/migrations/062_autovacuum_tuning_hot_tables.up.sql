-- Autovacuum agressivo nas tabelas quentes.
-- A retenção (cleanup_automation_runs / cleanup_action_history) e o churn de
-- dispatch_logs/webhook_events/ml_link_list_items geram muito dead-tuple; o
-- scale_factor default (0.2 = 20%) vacuuma tarde demais nessas tabelas grandes,
-- inflando índices de dedup e degradando plano. 0.05 (5%) mantém enxuto.
-- Aplicado ao vivo em 16/07/2026; esta migração torna DURÁVEL (sobrevive a
-- um init limpo do volume). Idempotente (ALTER ... SET reaplica sem efeito).
ALTER TABLE ml_link_list_items SET (autovacuum_vacuum_scale_factor = 0.05, autovacuum_analyze_scale_factor = 0.05, autovacuum_vacuum_cost_delay = 2);
ALTER TABLE dispatch_logs      SET (autovacuum_vacuum_scale_factor = 0.05, autovacuum_analyze_scale_factor = 0.05, autovacuum_vacuum_cost_delay = 2);
ALTER TABLE webhook_events     SET (autovacuum_vacuum_scale_factor = 0.05, autovacuum_analyze_scale_factor = 0.05, autovacuum_vacuum_cost_delay = 2);
ALTER TABLE automation_runs    SET (autovacuum_vacuum_scale_factor = 0.05, autovacuum_analyze_scale_factor = 0.05, autovacuum_vacuum_cost_delay = 2);
ALTER TABLE action_history     SET (autovacuum_vacuum_scale_factor = 0.05, autovacuum_analyze_scale_factor = 0.05, autovacuum_vacuum_cost_delay = 2);
