// Канал «Сообщить об ошибке» (FR-28). Story 1.9: mailto-stub. Story 5.4: полноценный POST на write-эндпоинт
// /api/error-reports (Directus-очередь) + mailto как no-JS фолбэк. [Source: epics.md:1576-1594; prd.md §5.11]
import type { components } from '../../shared/api/schema.gen';

export type ErrorReportRequest = components['schemas']['ErrorReportRequest'];
export type ErrorReportResponse = components['schemas']['ErrorReportResponse'];

// Адрес канала возражения (фолбэк-mailto; реальный — в деплое/ENV позже).
export const REPORT_ERROR_EMAIL = 'errors@ashyqqala.kz';

// reportErrorMailto — mailto со заполненной нейтральной темой (no-JS фолбэк, прогрессив-энхансмент).
export function reportErrorMailto(subject: string): string {
  return `mailto:${REPORT_ERROR_EMAIL}?subject=${encodeURIComponent(subject)}`;
}

// ReportTarget — контекст обращения (из карточки/флага/лота — НЕ вводится руками).
export interface ReportTarget {
  kind: ErrorReportRequest['kind'];
  subjectType: ErrorReportRequest['subject_type'];
  subjectRef: string;
  sourceUrl?: string;
}

// buildErrorReportRequest — ЧИСТАЯ сборка wire-запроса из контекста + полей формы (тестируется vitest без jsdom).
// trim'ит message/contact; пустые опц. поля → undefined (не шлём «»). website — honeypot (как есть, trimmed).
export function buildErrorReportRequest(
  target: ReportTarget,
  fields: { message: string; contact: string; leaveBlank: string },
): ErrorReportRequest {
  const contact = fields.contact.trim();
  const leaveBlank = fields.leaveBlank.trim();
  return {
    kind: target.kind,
    subject_type: target.subjectType,
    subject_ref: target.subjectRef,
    message: fields.message.trim(),
    ...(contact ? { contact } : {}),
    ...(target.sourceUrl ? { source_url: target.sourceUrl } : {}),
    ...(leaveBlank ? { leave_blank: leaveBlank } : {}),
  };
}

export interface ReportErrorFailure extends Error {
  status: number;
  code: string;
}

// postErrorReport — отправка обращения на write-эндпоинт. На !ok бросает с честным кодом
// (RATE_LIMITED/VALIDATION_FAILED/INTERNAL) → форма показывает честную ошибку, текст НЕ теряется.
export async function postErrorReport(req: ErrorReportRequest): Promise<ErrorReportResponse> {
  const res = await fetch('/api/error-reports', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as components['schemas']['Error'];
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`error-report failed: ${res.status} ${code}`) as ReportErrorFailure;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as ErrorReportResponse;
}
