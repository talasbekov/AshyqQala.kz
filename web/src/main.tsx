import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import './shared/tokens/tokens.css';
import { initTheme } from './shared/tokens/theme';

initTheme(); // применяет [data-theme] из сохранённого выбора / prefers-color-scheme

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('#root not found');
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
