-- name: GetLatestActByContractID :one
-- Последний акт по контракту (internal contract_id) для карточки (Story 5.1, FR-11). Детерминированный
-- порядок: наибольшая act_date (NULL — в конец), затем id. Не найдено → pgx.ErrNoRows (честное «нет акта»,
-- карточка показывает блок акта в состоянии no_data, НЕ выдумывает дату/подписанта).
SELECT id, contract_id, goszakup_act_id, act_date, signer_info, source_url, imported_at, updated_at
FROM acts
WHERE contract_id = $1
ORDER BY act_date DESC NULLS LAST, id DESC
LIMIT 1;
