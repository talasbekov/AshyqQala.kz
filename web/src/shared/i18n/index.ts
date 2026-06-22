// i18n-каркас (AC1): i18next namespace `chrome` (UI-строки платформы, ОТДЕЛЬНО от доменных данных).
// KZ по умолчанию, RU фолбэк. Язык — только kk/ru (НИКОГДА kz: kz=страна ISO3166, kk=язык ISO639-1).
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import kkChrome from './locales/kk/chrome.json';
import ruChrome from './locales/ru/chrome.json';

export type Lang = 'kk' | 'ru';
export const LANGS: readonly Lang[] = ['kk', 'ru'];
export const DEFAULT_LANG: Lang = 'kk';

const STORAGE_KEY = 'aq-lang';

export const resources = {
  kk: { chrome: kkChrome },
  ru: { chrome: ruChrome },
} as const;

function resolveInitialLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'kk' || saved === 'ru') return saved;
  } catch {
    // localStorage недоступен (приватный режим/SSR) — берём дефолт
  }
  return DEFAULT_LANG;
}

function syncHtmlLang(lng: string): void {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lng;
  }
}

i18n
  .use(initReactI18next)
  .init({
    resources,
    lng: resolveInitialLang(),
    fallbackLng: 'ru',
    defaultNS: 'chrome',
    ns: ['chrome'],
    interpolation: { escapeValue: false }, // React сам экранирует
  })
  .catch((e: unknown) => {
    // На статичных in-memory ресурсах init не должен падать; не глотаем молча.
    console.error('i18n init failed', e);
  });

syncHtmlLang(i18n.language);
i18n.on('languageChanged', syncHtmlLang);

// changeLanguage — сменить язык + персист (видимый toggle-UI — Story 1.7+).
export function changeLanguage(lang: Lang): void {
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    // запись недоступна — меняем без персиста
  }
  i18n.changeLanguage(lang).catch((e: unknown) => {
    console.error('changeLanguage failed', lang, e);
  });
}

export default i18n;
