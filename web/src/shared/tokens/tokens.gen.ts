// AUTO-GENERATED из web/tokens.json (make gen-tokens). НЕ править руками.

// Закрытый список семантических токенов (UX-DR2: primitive в фичах запрещены).
export type TokenName =
  | '--color-primary'
  | '--color-primary-strong'
  | '--color-primary-soft'
  | '--color-on-primary'
  | '--color-accent'
  | '--color-accent-soft'
  | '--color-on-accent'
  | '--color-signal-attention'
  | '--color-signal-attention-bg'
  | '--color-signal-attention-fg'
  | '--color-signal-attention-border'
  | '--color-confirmed'
  | '--color-confirmed-text'
  | '--color-confirmed-soft'
  | '--color-muted'
  | '--color-muted-text'
  | '--color-muted-soft'
  | '--color-surface'
  | '--color-surface-sunken'
  | '--color-border'
  | '--color-text-primary'
  | '--color-text-secondary'
  | '--color-link'
  | '--color-link-on-sunken'
  | '--color-focus-ring'
  | '--font-family-sans'
  | '--font-family-mono'
  | '--type-display-size'
  | '--type-display-weight'
  | '--type-display-line-height'
  | '--type-display-letter-spacing'
  | '--type-heading-size'
  | '--type-heading-weight'
  | '--type-heading-line-height'
  | '--type-heading-letter-spacing'
  | '--type-subheading-size'
  | '--type-subheading-weight'
  | '--type-subheading-line-height'
  | '--type-amount-size'
  | '--type-amount-weight'
  | '--type-amount-line-height'
  | '--type-amount-letter-spacing'
  | '--type-body-size'
  | '--type-body-weight'
  | '--type-body-line-height'
  | '--type-body-strong-size'
  | '--type-body-strong-weight'
  | '--type-body-strong-line-height'
  | '--type-meta-size'
  | '--type-meta-weight'
  | '--type-meta-line-height'
  | '--type-label-caps-size'
  | '--type-label-caps-weight'
  | '--type-label-caps-line-height'
  | '--type-label-caps-letter-spacing'
  | '--type-mono-size'
  | '--type-mono-weight'
  | '--type-mono-line-height'
  | '--radius-none'
  | '--radius-sm'
  | '--radius-md'
  | '--radius-default'
  | '--radius-card'
  | '--radius-lg'
  | '--radius-xl'
  | '--radius-full'
  | '--space-0'
  | '--space-1'
  | '--space-2'
  | '--space-3'
  | '--space-4'
  | '--space-5'
  | '--space-6'
  | '--space-8'
  | '--space-10'
  | '--space-gutter'
  | '--space-margin-mobile'
  | '--space-section-gap'
  | '--space-row-gap'
  | '--space-tap-target'
  | '--space-focus-ring-width'
  | '--space-focus-ring-offset';

export const TOKEN_NAMES: readonly TokenName[] = [
  '--color-primary',
  '--color-primary-strong',
  '--color-primary-soft',
  '--color-on-primary',
  '--color-accent',
  '--color-accent-soft',
  '--color-on-accent',
  '--color-signal-attention',
  '--color-signal-attention-bg',
  '--color-signal-attention-fg',
  '--color-signal-attention-border',
  '--color-confirmed',
  '--color-confirmed-text',
  '--color-confirmed-soft',
  '--color-muted',
  '--color-muted-text',
  '--color-muted-soft',
  '--color-surface',
  '--color-surface-sunken',
  '--color-border',
  '--color-text-primary',
  '--color-text-secondary',
  '--color-link',
  '--color-link-on-sunken',
  '--color-focus-ring',
  '--font-family-sans',
  '--font-family-mono',
  '--type-display-size',
  '--type-display-weight',
  '--type-display-line-height',
  '--type-display-letter-spacing',
  '--type-heading-size',
  '--type-heading-weight',
  '--type-heading-line-height',
  '--type-heading-letter-spacing',
  '--type-subheading-size',
  '--type-subheading-weight',
  '--type-subheading-line-height',
  '--type-amount-size',
  '--type-amount-weight',
  '--type-amount-line-height',
  '--type-amount-letter-spacing',
  '--type-body-size',
  '--type-body-weight',
  '--type-body-line-height',
  '--type-body-strong-size',
  '--type-body-strong-weight',
  '--type-body-strong-line-height',
  '--type-meta-size',
  '--type-meta-weight',
  '--type-meta-line-height',
  '--type-label-caps-size',
  '--type-label-caps-weight',
  '--type-label-caps-line-height',
  '--type-label-caps-letter-spacing',
  '--type-mono-size',
  '--type-mono-weight',
  '--type-mono-line-height',
  '--radius-none',
  '--radius-sm',
  '--radius-md',
  '--radius-default',
  '--radius-card',
  '--radius-lg',
  '--radius-xl',
  '--radius-full',
  '--space-0',
  '--space-1',
  '--space-2',
  '--space-3',
  '--space-4',
  '--space-5',
  '--space-6',
  '--space-8',
  '--space-10',
  '--space-gutter',
  '--space-margin-mobile',
  '--space-section-gap',
  '--space-row-gap',
  '--space-tap-target',
  '--space-focus-ring-width',
  '--space-focus-ring-offset',
];

/** token('--color-primary') → 'var(--color-primary)'. Только валидные семантические имена. */
export function token(name: TokenName): string {
  return `var(${name})`;
}
