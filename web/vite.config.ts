import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

// alias @fixtures → ../fixtures (ОБЩИЕ фикстуры монорепо; e2e/тесты читают отсюда)
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@fixtures': fileURLToPath(new URL('../fixtures', import.meta.url)),
    },
  },
});
