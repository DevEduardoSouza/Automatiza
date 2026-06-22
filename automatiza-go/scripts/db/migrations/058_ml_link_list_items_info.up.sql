-- Per-item "info" text for ML link list items.
--
-- The dispatch template already supports an {info} variable (the free-form
-- line populated by the info[...] tag in offer_generate). In LIST mode the
-- items come from a pre-registered list with no trigger message, so {info}
-- rendered empty and the line collapsed.
--
-- This column lets operators provide that text per item — chiefly via the
-- spreadsheet import (a second "info" column), so the {info} line shows in
-- list-mode dispatches the same way it does in offer_generate.
--
-- NULL = no info (legacy and default), {info} renders empty as before.

ALTER TABLE ml_link_list_items
ADD COLUMN IF NOT EXISTS info TEXT;
