-- Синтетический seed РНУ (Story 5.2, FR-14; реальный — живой /v2/rnu-импорт Epic 2). Активность метки
-- реконструируется на ЧТЕНИИ из дат (авто-снятие по end_date), поэтому seed задаёт обе ветки:
--   222… — активная (end_date NULL); 333… — истёкшая (end_date в прошлом) → метка снята авто.
-- Идемпотентно через NOT EXISTS по goszakup_rnu_id (на rnu_entries нет UNIQUE — наполнение управляется импортёром).
INSERT INTO rnu_entries (organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url)
SELECT o.id, 'RNU-A-1', DATE '2026-01-15', NULL,
       'Реестр недобросовестных участников госзакупок', 'https://goszakup.gov.kz/ru/registry/rnu'
FROM organizations o
WHERE o.bin = '222222222222'
  AND NOT EXISTS (SELECT 1 FROM rnu_entries e WHERE e.goszakup_rnu_id = 'RNU-A-1');

INSERT INTO rnu_entries (organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url)
SELECT o.id, 'RNU-B-1', DATE '2024-01-01', DATE '2025-01-01',
       'Реестр недобросовестных участников госзакупок', 'https://goszakup.gov.kz/ru/registry/rnu'
FROM organizations o
WHERE o.bin = '333333333333'
  AND NOT EXISTS (SELECT 1 FROM rnu_entries e WHERE e.goszakup_rnu_id = 'RNU-B-1');
