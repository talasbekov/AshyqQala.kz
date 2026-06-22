// Типы для tokens-codegen.mjs (codegen — JS-скрипт, запускается node; типы — для tsc/тестов).
export interface TokenLeaf {
  name: string;
  light: string;
  dark: string | null;
}
export function flatten(tokens: unknown): TokenLeaf[];
export function generate(tokens: unknown): { css: string; ts: string };
