-- name: InsertErrorReport :one
-- Принимает публичное обращение об ошибке (FR-28, Story 5.4) в очередь error_reports (status=new по умолчанию).
-- Только INSERT (публичный API не читает/не правит очередь — это Directus). contact/source_url опциональны (NULL).
-- Возвращает id (для honest-ответа «принято #id», без публичного трекинга статуса — PRD §5.11).
INSERT INTO error_reports (kind, subject_type, subject_ref, message, contact, source_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at;

-- name: GetErrorReport :one
-- Чтение обращения по id (для интеграционных тестов/диагностики; публичный API НЕ использует). Нет → pgx.ErrNoRows.
SELECT id, kind, subject_type, subject_ref, message, contact, source_url, status, created_at, resolved_at
FROM error_reports
WHERE id = $1;
