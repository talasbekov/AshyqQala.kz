import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import './shared/tokens/tokens.css';
import './index.css'; // глобальный base + chrome-каркас (шапка/дом/404) на семантических токенах
import './shared/i18n'; // инициализация i18next (chrome ns, kk-дефолт) + синк <html lang>
import { initTheme } from './shared/tokens/theme';
import { queryClient } from './app/queryClient';
import { router } from './router';

initTheme(); // применяет [data-theme] из сохранённого выбора / prefers-color-scheme

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('#root not found');
}

createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
