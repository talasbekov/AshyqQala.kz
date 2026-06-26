import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  buildErrorReportRequest,
  postErrorReport,
  reportErrorMailto,
  type ReportTarget,
} from './reportError';
import { resources, LANGS } from '../../shared/i18n';

const target: ReportTarget = {
  kind: 'flag_error',
  subjectType: 'contract',
  subjectRef: 'DEMO-0002',
  sourceUrl: 'https://goszakup.gov.kz/ru/contract/DEMO-0002',
};

describe('buildErrorReportRequest (Story 5.4: сборка wire-запроса)', () => {
  it('trim message/contact; пустой contact → НЕ отправляется (не «»)', () => {
    const req = buildErrorReportRequest(target, { message: '  завышено  ', contact: '   ', leaveBlank: '' });
    expect(req.message).toBe('завышено');
    expect('contact' in req).toBe(false);
    expect(req.source_url).toBe(target.sourceUrl);
    expect('leave_blank' in req).toBe(false);
    expect(req.kind).toBe('flag_error');
    expect(req.subject_type).toBe('contract');
    expect(req.subject_ref).toBe('DEMO-0002');
  });

  it('contact и honeypot включаются, когда непусты', () => {
    const req = buildErrorReportRequest(target, { message: 'm', contact: ' me@x.kz ', leaveBlank: 'bot' });
    expect(req.contact).toBe('me@x.kz');
    expect(req.leave_blank).toBe('bot');
  });

  it('без sourceUrl поле source_url не добавляется', () => {
    const req = buildErrorReportRequest(
      { kind: 'geo_wrong_point', subjectType: 'geo_object', subjectRef: 'LOT-1' },
      { message: 'm', contact: '', leaveBlank: '' },
    );
    expect('source_url' in req).toBe(false);
    expect(req.kind).toBe('geo_wrong_point');
  });
});

describe('reportErrorMailto (no-JS фолбэк)', () => {
  it('mailto с экранированной темой', () => {
    const href = reportErrorMailto('Ошибка #DEMO-0002');
    expect(href.startsWith('mailto:errors@ashyqqala.kz?subject=')).toBe(true);
    expect(href).toContain(encodeURIComponent('Ошибка #DEMO-0002'));
  });
});

describe('postErrorReport (клиент write-эндпоинта)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('успех → распарсенный ответ', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({ ok: true, json: async () => ({ id: 7, status: 'received' }) })),
    );
    const r = await postErrorReport(buildErrorReportRequest(target, { message: 'm', contact: '', leaveBlank: '' }));
    expect(r).toEqual({ id: 7, status: 'received' });
  });

  it('!ok → бросает с кодом из тела (честная ошибка для формы)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: false,
        status: 429,
        json: async () => ({ error: { code: 'RATE_LIMITED', message: 'too many' } }),
      })),
    );
    await expect(
      postErrorReport(buildErrorReportRequest(target, { message: 'm', contact: '', leaveBlank: '' })),
    ).rejects.toMatchObject({ status: 429, code: 'RATE_LIMITED' });
  });
});

describe('i18n: report_error.* парны kk↔ru (нет тихого ru-фолбэка для kk)', () => {
  const flatten = (obj: unknown, prefix = '', out: string[] = []): string[] => {
    if (obj && typeof obj === 'object') {
      for (const [k, v] of Object.entries(obj)) flatten(v, prefix ? `${prefix}.${k}` : k, out);
    } else {
      out.push(prefix);
    }
    return out;
  };
  it('наборы ключей report_error идентичны в kk и ru', () => {
    const keysByLang = LANGS.map((lang) => {
      const re = (resources[lang].chrome as Record<string, unknown>).report_error;
      return new Set(flatten(re).sort());
    });
    const [kk, ru] = keysByLang;
    expect([...kk].sort()).toEqual([...ru].sort());
    // несущие новые ключи присутствуют
    for (const lang of LANGS) {
      const re = (resources[lang].chrome as Record<string, unknown>).report_error as Record<string, unknown>;
      const form = re.form as Record<string, string>;
      expect(typeof re.geo_link, `${lang}:geo_link`).toBe('string');
      for (const k of ['title', 'message_label', 'submit', 'submitting', 'success', 'error', 'mailto_fallback']) {
        expect(typeof form[k], `${lang}:form.${k}`).toBe('string');
      }
    }
  });
});
