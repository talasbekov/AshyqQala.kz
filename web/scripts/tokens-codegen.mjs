// Свой codegen дизайн-токенов (UX-DR1): tokens.json (W3C DTCG) → tokens.css + tokens.gen.ts.
// БЕЗ Style Dictionary/Tailwind, ноль зависимостей (только node:* stdlib). Вывод ДЕТЕРМИНИРОВАН
// (стабильный порядок обхода) — для golden-снапшота и CI-стража generated==regenerated.
import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const BANNER_CSS =
  '/* AUTO-GENERATED из web/tokens.json (make gen-tokens). НЕ править руками. */\n';
const BANNER_TS = '// AUTO-GENERATED из web/tokens.json (make gen-tokens). НЕ править руками.\n';

// flatten — обходит дерево токенов; лист = узел с $value. Имя CSS-переменной = путь через "-".
// Тёмное значение цвета берётся из $extensions["aq.dark"].
export function flatten(tokens) {
  const out = [];
  const walk = (node, path) => {
    if (node && typeof node === 'object' && '$value' in node) {
      const ext = node.$extensions || {};
      const dark = ext['aq.dark'];
      out.push({
        name: '--' + path.join('-'),
        light: String(node.$value),
        dark: dark != null ? String(dark) : null,
      });
      return;
    }
    if (node && typeof node === 'object') {
      for (const key of Object.keys(node)) {
        if (key.startsWith('$')) continue;
        walk(node[key], [...path, key]);
      }
    }
  };
  for (const key of Object.keys(tokens)) {
    if (key.startsWith('$')) continue;
    walk(tokens[key], [key]);
  }
  return out;
}

// generate — чистая функция: токены → { css, ts }. Без IO (тестируется напрямую).
export function generate(tokens) {
  const leaves = flatten(tokens);
  const root = leaves.map((l) => `  ${l.name}: ${l.light};`).join('\n');
  const dark = leaves
    .filter((l) => l.dark !== null)
    .map((l) => `  ${l.name}: ${l.dark};`)
    .join('\n');
  const css =
    BANNER_CSS + ':root {\n' + root + '\n}\n\n' + '[data-theme="dark"] {\n' + dark + '\n}\n';

  const names = leaves.map((l) => l.name);
  const union = names.map((n) => `  | '${n}'`).join('\n');
  const arr = names.map((n) => `  '${n}',`).join('\n');
  const ts =
    BANNER_TS +
    '\n' +
    '// Закрытый список семантических токенов (UX-DR2: primitive в фичах запрещены).\n' +
    'export type TokenName =\n' +
    union +
    ';\n\n' +
    'export const TOKEN_NAMES: readonly TokenName[] = [\n' +
    arr +
    '\n];\n\n' +
    "/** token('--color-primary') → 'var(--color-primary)'. Только валидные семантические имена. */\n" +
    'export function token(name: TokenName): string {\n  return `var(${name})`;\n}\n';

  return { css, ts };
}

// Запуск как скрипт (node scripts/tokens-codegen.mjs) — пишет файлы. При импорте (тест) — no-op.
const invokedDirectly =
  process.argv[1] && realpathSync(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedDirectly) {
  const webRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
  const tokens = JSON.parse(readFileSync(join(webRoot, 'tokens.json'), 'utf8'));
  const { css, ts } = generate(tokens);
  writeFileSync(join(webRoot, 'src/shared/tokens/tokens.css'), css);
  writeFileSync(join(webRoot, 'src/shared/tokens/tokens.gen.ts'), ts);
  console.log('tokens-codegen: tokens.css + tokens.gen.ts обновлены');
}
