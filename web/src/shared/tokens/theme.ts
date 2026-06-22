// Минимальный bootstrap темы (AC3): применяет [data-theme] из сохранённого выбора, иначе из
// prefers-color-scheme. Компоненты тему НЕ знают — читают только семантические токены.
// Видимый toggle-UI — позже (нужен chrome/хедер, Story 1.7+).
export type Theme = 'light' | 'dark';

const STORAGE_KEY = 'aq-theme';

export function resolveInitialTheme(): Theme {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'light' || saved === 'dark') return saved;
  } catch {
    // localStorage недоступен (приватный режим/SSR) — игнорируем, идём к prefers-color-scheme
  }
  if (
    typeof window !== 'undefined' &&
    window.matchMedia?.('(prefers-color-scheme: dark)').matches
  ) {
    return 'dark';
  }
  return 'light';
}

export function applyTheme(theme: Theme): void {
  if (typeof document === 'undefined') return; // SSR/node — нет DOM, no-op (симметрично window-guard)
  document.documentElement.setAttribute('data-theme', theme);
}

export function setTheme(theme: Theme): void {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // запись недоступна — применяем тему без персиста
  }
  applyTheme(theme);
}

// initTheme — вызывается один раз на старте приложения (main.tsx).
export function initTheme(): Theme {
  const theme = resolveInitialTheme();
  applyTheme(theme);
  return theme;
}
