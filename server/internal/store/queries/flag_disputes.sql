-- name: UpsertFlagDispute :exec
-- Идемпотентно фиксирует/обновляет диспут флага (один на risk_flag_id, AR-28). resolved_at вычисляется из
-- статуса: NULL для raised/disputed, now() для confirmed/withdrawn (CHECK flag_disputes_resolved_chk). Повтор
-- того же risk_flag_id → UPDATE статуса/заметки (не дубль). note/source_url через COALESCE: пустой вход (NULL)
-- СОХРАНЯЕТ ранее записанное обоснование (а не затирает — иначе повторный resolve без заметки терял бы контекст).
INSERT INTO flag_disputes (risk_flag_id, flag_type, subject_type, subject_id, status, note, source_url, resolved_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, CASE WHEN $5 IN ('confirmed', 'withdrawn') THEN now() ELSE NULL END)
ON CONFLICT (risk_flag_id) DO UPDATE SET
    status      = EXCLUDED.status,
    note        = COALESCE(EXCLUDED.note, flag_disputes.note),
    source_url  = COALESCE(EXCLUDED.source_url, flag_disputes.source_url),
    resolved_at = EXCLUDED.resolved_at;

-- name: GetFlagDispute :one
-- Диспут по флагу (для чтения/тестов). Не найдено → pgx.ErrNoRows.
SELECT id, risk_flag_id, flag_type, subject_type, subject_id, status, note, source_url, created_at, resolved_at
FROM flag_disputes
WHERE risk_flag_id = $1;

-- name: CountFlagDisputesByStatus :one
-- Число диспутов заданного статуса (питает SM-C1: confirmed|withdrawn — знаменатель, withdrawn — числитель).
SELECT count(*) AS n FROM flag_disputes WHERE status = $1;
