-- Migration: 051_ml_link_list_items_drop_unique_url.up.sql
-- Drops the UNIQUE (list_id, product_url) constraint on ml_link_list_items.
--
-- Why: o cliente importa planilhas onde a ordem das linhas e a repetição
-- de produtos são significativas (mesmo link aparece N vezes = N envios).
-- O constraint impedia esse caso de uso — INSERT bulk com qualquer
-- duplicata fazia toda a transação abortar e o usuário perdia a planilha
-- inteira (500 em produção, reverted via hotfix em 2026-05-28).
--
-- Tem dois nomes possíveis dependendo do schema-base de cada env, o 022
-- adicionou condicionalmente. Aqui dropamos os dois com IF EXISTS.
--
-- Itens existentes NÃO são afetados — só o constraint sai.
--
-- O trigger sync_urls_trigger (copia origin_url <-> product_url) continua,
-- agora puramente como sync de dados sem efeito colateral de UNIQUE.

BEGIN;

ALTER TABLE ml_link_list_items
    DROP CONSTRAINT IF EXISTS ml_link_list_items_list_id_product_url_key;

ALTER TABLE ml_link_list_items
    DROP CONSTRAINT IF EXISTS ml_link_list_items_unique_origin_url_per_list;

COMMIT;
