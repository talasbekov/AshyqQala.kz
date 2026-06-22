import { describe, it, expect, vi, afterEach } from 'vitest';
import { fetchContract } from './useContract';

const g = globalThis as unknown as { fetch?: unknown };
afterEach(() => {
  delete g.fetch;
});

describe('fetchContract', () => {
  it('возвращает контракт при 200', async () => {
    g.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ goszakup_contract_id: 'DEMO-0001' }),
    });
    const c = await fetchContract('DEMO-0001');
    expect(c.goszakup_contract_id).toBe('DEMO-0001');
  });

  it('бросает с честным кодом NOT_FOUND при 404', async () => {
    g.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: { code: 'NOT_FOUND', message: 'x' } }),
    });
    await expect(fetchContract('NOPE')).rejects.toMatchObject({ status: 404, code: 'NOT_FOUND' });
  });

  it('INTERNAL при не-JSON теле ошибки', async () => {
    g.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json')),
    });
    await expect(fetchContract('x')).rejects.toMatchObject({ status: 500, code: 'INTERNAL' });
  });
});
