import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// PermalinkButton — копирует перманентную ссылку-на-дату (AR-29) в буфер обмена. href — относительный путь
// со штампом (as_of, mv); копируется АБСОЛЮТНЫЙ URL (origin + href), чтобы ссылка пережила пересылку в
// мессенджер/СМИ (SM-5). При отказе clipboard (HTTP/старый браузер) — честный фолбэк: показываем кликабельную
// ссылку для ручного копирования (не тихий no-op). Нейтральная лексика; без аккаунта.
export function PermalinkButton({ href }: { href: string }) {
  const { t } = useTranslation('chrome');
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle');
  const abs = typeof window !== 'undefined' ? window.location.origin + href : href;

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(abs);
      setState('copied');
    } catch {
      setState('failed');
    }
  };

  return (
    <div className="contract-permalink">
      <button type="button" className="contract-permalink__btn" onClick={onCopy}>
        {state === 'copied' ? t('methodology.permalink_copied') : t('methodology.permalink')}
      </button>
      {state === 'failed' ? (
        <span className="contract-permalink__fallback">
          {t('methodology.permalink_failed')}:{' '}
          <a href={href}>{abs}</a>
        </span>
      ) : null}
    </div>
  );
}
