-- +goose Up
-- error_reports — публичные БЕЗАККАУНТНЫЕ обращения граждан об ошибке в данных/флаге/геопривязке (FR-28, Story 5.4).
-- Несущее юр-ограждение (§7.1): досудебный канал возражения. «Запись падает в Directus-очередь» (AC-1): публичный
-- API только INSERT, Directus читает/триажит (гранты — compose). Importer без доступа (architecture.md:152,416).
-- Паттерн flag_disputes (AR-4): soft-ref субъект ТЕКСТОМ (без FK — проекции/импорт перестраиваются), денорм + CHECK-enum.
-- resolved_at заполнен ⟺ финальный статус (resolved|dismissed) — честный инвариант (как flag_disputes_resolved_chk).
CREATE TABLE error_reports (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind          TEXT NOT NULL,               -- data_error|flag_error|geo_wrong_point (AR-28 «точка не там»)
    subject_type  TEXT NOT NULL,               -- contract|contractor|geo_object
    subject_ref   TEXT NOT NULL,               -- soft-ref: goszakup_contract_id|flag_id|lot/geo id (без FK, AR-4)
    message       TEXT NOT NULL,               -- «что не так» (обязательно)
    contact       TEXT,                        -- контакт по желанию (опционально — без аккаунта, §7.2)
    source_url    TEXT,                         -- страница/первоисточник, где замечена ошибка (опц.)
    status        TEXT NOT NULL DEFAULT 'new', -- new|triaged|resolved|dismissed (триаж в Directus)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ,                 -- заполнен ⟺ resolved|dismissed
    CONSTRAINT error_reports_kind_chk CHECK (kind IN ('data_error', 'flag_error', 'geo_wrong_point')),
    CONSTRAINT error_reports_subject_type_chk CHECK (subject_type IN ('contract', 'contractor', 'geo_object')),
    CONSTRAINT error_reports_status_chk CHECK (status IN ('new', 'triaged', 'resolved', 'dismissed')),
    -- message не пустой/не только пробелы (честность: пустое обращение — не обращение; страховка к серверной валидации).
    CONSTRAINT error_reports_message_nonempty_chk CHECK (length(btrim(message)) > 0),
    -- resolved ⟺ финальный статус ⟺ resolved_at заполнен (иначе скрытая рассинхронизация очереди триажа).
    CONSTRAINT error_reports_resolved_chk CHECK ((status IN ('resolved', 'dismissed')) = (resolved_at IS NOT NULL))
);
-- Очередь триажа Directus: свежие new сверху.
CREATE INDEX error_reports_status_idx ON error_reports (status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS error_reports;
