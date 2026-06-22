import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import globals from 'globals';
import prettier from 'eslint-config-prettier';

export default tseslint.config(
  { ignores: ['dist', 'node_modules'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: { ...globals.browser },
    },
  },
  {
    // Node-контекст: конфиги сборки/тулинга и codegen-скрипты (вне браузера)
    files: ['*.{js,cjs,mjs,ts}', 'vite.config.ts', 'scripts/**/*.{js,cjs,mjs}'],
    languageOptions: {
      globals: { ...globals.node },
    },
  },
  prettier,
);
