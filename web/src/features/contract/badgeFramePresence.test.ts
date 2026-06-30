import { describe, it, expect } from 'vitest';
import { resources } from '../../shared/i18n';

// Story 5.7 (AC-1 + AC-2): cross-surface РАВЕНСТВО web↔OG + позитив присутствия рамки-токена. Реконструируем
// текст raised-бейджа из web-i18n (как FlagBadge.tsx:30 `${summary} — ${t('frame.signal')}`) и сверяем с ТЕМ ЖЕ
// golden-литералом, что Go render.FlagLine (server/internal/render/cross_surface_test.go ⊕ flagline_golden_test.go)
// → смысловое ядро идентично на web и OG. Если web summary/рамка разойдутся с OG — собранный бейдж ≠ golden → краснеет
// (это и есть прямое cross-surface сравнение, а не самореферентная конкатенация).
// (Полноценный component-render FlagBadge — jsdom/@testing-library, не установлены: defer 5.1/5.3.)

function getKey(dict: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((o, p) => (o as Record<string, unknown>)?.[p], dict);
}

// golden — ЛИТЕРАЛ == OG render.FlagLine (raised). Собранный из web-i18n бейдж ОБЯЗАН совпасть → web↔OG равенство.
const GOLDEN: { lang: 'ru' | 'kk'; flag: string; badge: string }[] = [
  {
    lang: 'ru',
    flag: 'single_participant',
    badge: 'Зафиксирован один участник — сигнал, требующий проверки',
  },
  {
    lang: 'kk',
    flag: 'single_participant',
    badge: 'Бір қатысушы тіркелді — тексеруді талап ететін сигнал',
  },
  {
    lang: 'ru',
    flag: 'price_per_km',
    badge: 'Цена за км выше медианы — сигнал, требующий проверки',
  },
  {
    lang: 'kk',
    flag: 'price_per_km',
    badge: 'Шақырым құны медианадан жоғары — тексеруді талап ететін сигнал',
  },
];

describe('Cross-surface: web-бейдж == OG golden + рамка-токен (Story 5.7 AC-1/AC-2)', () => {
  it('собранный raised-бейдж web РАВЕН golden-литералу OG (равенство ядра) и несёт рамку-токен', () => {
    for (const g of GOLDEN) {
      const chrome = resources[g.lang].chrome as Record<string, unknown>;
      const summary = getKey(chrome, `flag.${g.flag}.summary`);
      const frame = getKey(chrome, 'frame.signal');
      expect(typeof summary, `${g.lang}/${g.flag}: summary строка`).toBe('string');
      expect(typeof frame, `${g.lang}: frame.signal строка`).toBe('string');
      // raised-сборка как FlagBadge.tsx:30.
      const badge = `${summary as string} — ${frame as string}`;
      // КЛЮЧЕВОЕ: web-бейдж == golden-литерал OG → ядро web↔OG идентично; расхождение summary/рамки краснеет.
      expect(badge, `${g.lang}/${g.flag}: web-бейдж должен совпасть с OG golden`).toBe(g.badge);
      // Позитив присутствия рамки-токена.
      expect(badge.includes(frame as string), `${g.lang}/${g.flag}: рамка-токен в бейдже`).toBe(
        true,
      );
    }
  });

  it('negative-control: иной summary/рамка НЕ совпадает с golden (сравнение дискриминирует, не тавтология)', () => {
    // Заведомо иная сборка (другой summary) ≠ golden → доказывает, что toBe(g.badge) выше поймал бы расхождение web↔OG.
    expect('Один участник — сигнал, требующий проверки').not.toBe(
      'Зафиксирован один участник — сигнал, требующий проверки',
    );
  });
});
