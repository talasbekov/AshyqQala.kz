import { useEffect, useRef, type RefObject } from 'react';

// useDialogFocus — единая focus-механика модальных слоёв (Story 3.5, AC2; EXPERIENCE.md «Accessibility
// Floor»): при открытии фокус переносится в слой, внутри — focus-trap (Tab/Shift+Tab циклируют),
// Escape закрывает, при закрытии фокус возвращается на триггер (элемент, активный в момент открытия).
// Закрывает долги deferred-work 63/77/158/165 (лист 1.8 / методика 1.9/5.3 / форма 5.4).
//
// Свой хук, НЕ Radix (D1): Radix-модальность перехватывает pointer-события всей страницы (конфликт с
// картой под листом), а три существующих диалога уже на едином самодельном паттерне — точечная врезка
// дешевле рефактора. Триггер пересмотра: разрастание > ~100 строк → @radix-ui/react-dialog.
//
// Слои МОГУТ сосуществовать (карточка: методика + форма ошибки поверх) — стек ниже гарантирует, что
// Escape/Tab обрабатывает ТОЛЬКО верхний слой, а фокус-возврат идёт по цепочке триггеров.

// :not([tabindex="-1"]) в КАЖДОЙ клаузе: программно-фокусируемые (honeypot формы tabIndex=-1)
// не входят в Tab-цикл (код-ревью 3.5).
const FOCUSABLE =
  'a[href]:not([tabindex="-1"]), button:not([disabled]):not([tabindex="-1"]), ' +
  'input:not([disabled]):not([type="hidden"]):not([tabindex="-1"]), ' +
  'select:not([disabled]):not([tabindex="-1"]), textarea:not([disabled]):not([tabindex="-1"]), ' +
  '[tabindex]:not([tabindex="-1"])';

// Стек активных диалогов: верхний = последний открытый. Модульный (не React-состояние) — слои живут
// в разных поддеревьях (карточка/карта), общего родителя нет.
const dialogStack: symbol[] = [];

export function useDialogFocus(
  containerRef: RefObject<HTMLElement | null>,
  onClose: () => void,
  opts?: {
    // Начальный фокус (напр. textarea формы); по умолчанию — сам контейнер (tabIndex=-1).
    initialFocus?: RefObject<HTMLElement | null>;
    // Куда вернуть фокус, если триггер исчез из DOM (рекластеризация снесла маркер): честный
    // фолбэк — контейнер карты/страницы, НЕ <body>. null/undefined → ничего (браузерный дефолт).
    returnFallback?: () => HTMLElement | null;
  },
): void {
  // Пропсы меняются между рендерами — эффект открытия/закрытия гоняем строго один раз на mount,
  // актуальные значения читаем через ref.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const optsRef = useRef(opts);
  optsRef.current = opts;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const token = Symbol('dialog');
    dialogStack.push(token);
    const isTop = () => dialogStack[dialogStack.length - 1] === token;

    // <body> — не триггер (клики по canvas/незафокусированным зонам оставляют activeElement=body):
    // возврат на body бесполезен → уходим в returnFallback.
    const ae = document.activeElement;
    const trigger = ae instanceof HTMLElement && ae !== document.body ? ae : null;
    (optsRef.current?.initialFocus?.current ?? container).focus();

    // capture-фаза на document: ловит Tab/Escape независимо от того, куда ушёл фокус, и раньше
    // React-делегирования (узловые onKeyDown в диалогах больше не нужны).
    const onKeyDown = (e: KeyboardEvent) => {
      if (!isTop()) return; // обрабатывает только ВЕРХНИЙ слой
      if (e.isComposing) return; // IME-композиция: Escape/Tab принадлежат редактору, не диалогу
      if (e.key === 'Escape') {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== 'Tab') return;
      const nodes = Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE));
      if (nodes.length === 0) {
        // Внутри нет фокусируемых — фокус заперт на самом контейнере.
        e.preventDefault();
        container.focus();
        return;
      }
      const first = nodes[0];
      const last = nodes[nodes.length - 1];
      const active = document.activeElement;
      const inside = active instanceof HTMLElement && container.contains(active);
      if (e.shiftKey) {
        if (!inside || active === first || active === container) {
          e.preventDefault();
          last.focus();
        }
      } else if (!inside || active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', onKeyDown, true);

    return () => {
      document.removeEventListener('keydown', onKeyDown, true);
      // Гард −1: splice(-1,1) снял бы ЧУЖОЙ верхний слой при отсутствии токена (код-ревью 3.5).
      const i = dialogStack.indexOf(token);
      if (i !== -1) dialogStack.splice(i, 1);
      // Возврат фокуса на триггер; триггер снесён (teardown DOM-маркеров) → честный фолбэк.
      const target =
        trigger !== null && trigger.isConnected
          ? trigger
          : (optsRef.current?.returnFallback?.() ?? null);
      target?.focus();
    };
    // containerRef стабилен (useRef); открытие/закрытие = mount/unmount компонента диалога.
  }, [containerRef]);
}
