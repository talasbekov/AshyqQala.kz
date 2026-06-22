// Корневой layout (walking skeleton, Story 1.7): шапка + <Outlet/> для маршрутов.
import { Outlet, Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function App() {
  const { t } = useTranslation('chrome');
  return (
    <>
      <header className="app-header">
        <Link to="/">{t('app.title')}</Link>
      </header>
      <main>
        <Outlet />
      </main>
    </>
  );
}

// Индекс-страница: ссылка на демонстрационный контракт (сквозной путь DoD).
export function HomeView() {
  const { t } = useTranslation('chrome');
  return (
    <section className="home">
      <p>{t('app.tagline')}</p>
      <Link to="/contracts/DEMO-0001">{t('home.demo_link')}</Link>
    </section>
  );
}
