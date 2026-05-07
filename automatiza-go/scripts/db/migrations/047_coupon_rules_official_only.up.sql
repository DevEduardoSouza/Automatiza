-- official_only filter on coupon_rules.
--
-- Shopee Mall (lojas oficiais — Samsung, Mondial, etc) is the only
-- "trust signal" the affiliate API exposes per product. Operators
-- want a way to pin certain coupons to those products only — e.g.
-- a R$50 OFF coupon that only kicks in on official stores so the
-- channel doesn't burn it on noisier marketplace listings.
--
-- The matcher reads ShopeeOffer.IsMall (populated from productOfferV2's
-- shopType field after migration 047 ships) and skips rules whose
-- official_only=true don't match. Default FALSE preserves every
-- existing rule's behavior — they keep matching any product.

ALTER TABLE coupon_rules
ADD COLUMN IF NOT EXISTS official_only BOOLEAN NOT NULL DEFAULT FALSE;
