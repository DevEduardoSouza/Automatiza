-- Migration: 022_ml_link_list_items_unique_constraint.down.sql
-- Removes UNIQUE constraint on (list_id, origin_url)

ALTER TABLE ml_link_list_items
    DROP CONSTRAINT IF EXISTS ml_link_list_items_unique_origin_url_per_list;
