// <LangValue> (AC2): билингв доменное значение с пометкой языка в DOM + бейдж фолбэка.
// Строится против СПЕЦ-формы конверта {value, lang, is_fallback, resolution} — живые данные
// (wire-конверт) подключатся позже; машинный перевод в MVP запрещён (нет resolution=machine_translated).
import { useTranslation } from 'react-i18next';

export type Lang = 'kk' | 'ru';
export type Resolution = 'original_kk' | 'source_ru_shown_as_fallback';

export interface LangValueData {
  value: string;
  lang: Lang;
  is_fallback?: boolean;
  resolution?: Resolution;
}

export function LangValue({ data }: { data: LangValueData }) {
  const { t } = useTranslation('chrome');
  // Бейдж — chrome-текст (язык UI), поэтому ВНЕ value-span с lang={data.lang}: иначе он унаследует
  // язык значения и скринридер озвучит бейдж неверно (a11y).
  return (
    <>
      <span lang={data.lang} className="lang-value">
        {data.value}
      </span>
      {data.is_fallback ? (
        <span className="lang-value__badge"> ({t('lang.ru_source')})</span>
      ) : null}
    </>
  );
}
