import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { resolveInitialTheme, applyTheme, setTheme, initTheme } from './theme';

// vitest по умолчанию в node-окружении (нет DOM) — стабим браузерные globals точечно.
const g = globalThis as unknown as {
  localStorage?: unknown;
  window?: unknown;
  document?: unknown;
};

function stubStorage(initial: Record<string, string> = {}) {
  const store = { ...initial };
  g.localStorage = {
    getItem: (k: string) => (k in store ? store[k] : null),
    setItem: (k: string, v: string) => {
      store[k] = v;
    },
  };
}

function stubMatchMedia(prefersDark: boolean) {
  g.window = { matchMedia: () => ({ matches: prefersDark }) };
}

let attr: string | null = null;
function stubDocument() {
  attr = null;
  g.document = { documentElement: { setAttribute: (_: string, v: string) => (attr = v) } };
}

beforeEach(() => {
  attr = null;
});
afterEach(() => {
  delete g.localStorage;
  delete g.window;
  delete g.document;
});

describe('theme bootstrap', () => {
  it('сохранённый выбор имеет приоритет над prefers-color-scheme', () => {
    stubStorage({ 'aq-theme': 'dark' });
    stubMatchMedia(false);
    expect(resolveInitialTheme()).toBe('dark');
  });

  it('без сохранённого — берёт prefers-color-scheme', () => {
    stubStorage();
    stubMatchMedia(true);
    expect(resolveInitialTheme()).toBe('dark');
  });

  it('дефолт light, если ничего не задано', () => {
    stubStorage();
    stubMatchMedia(false);
    expect(resolveInitialTheme()).toBe('light');
  });

  it('некорректное сохранённое значение игнорируется', () => {
    stubStorage({ 'aq-theme': 'neon' });
    stubMatchMedia(true);
    expect(resolveInitialTheme()).toBe('dark');
  });

  it('localStorage недоступен → не падает, фолбэк на prefers', () => {
    g.localStorage = {
      getItem: () => {
        throw new Error('denied');
      },
    };
    stubMatchMedia(true);
    expect(resolveInitialTheme()).toBe('dark');
  });

  it('applyTheme без document (node/SSR) — no-op, не бросает', () => {
    expect(() => applyTheme('dark')).not.toThrow();
  });

  it('applyTheme ставит [data-theme] при наличии document', () => {
    stubDocument();
    applyTheme('dark');
    expect(attr).toBe('dark');
  });

  it('setTheme персистит и применяет; initTheme резолвит+применяет', () => {
    const setItem = vi.fn();
    g.localStorage = { getItem: () => null, setItem };
    stubMatchMedia(false);
    stubDocument();
    setTheme('dark');
    expect(setItem).toHaveBeenCalledWith('aq-theme', 'dark');
    expect(attr).toBe('dark');
    expect(initTheme()).toBe('light'); // getItem→null, prefers=false → light
    expect(attr).toBe('light');
  });
});
