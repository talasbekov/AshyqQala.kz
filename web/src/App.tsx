// Корневой layout (walking skeleton, Story 1.7): шапка + <Outlet/> для маршрутов.
import { Outlet, Link, useRouteError } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function App() {
  const { t } = useTranslation('chrome');
  return (
    <>
      <header className="app-header">
        <Link to="/">{t('app.title')}</Link>
        <nav className="app-nav">
          <Link to="/search">{t('search.nav')}</Link>
          <Link to="/map">{t('map.title')}</Link>
        </nav>
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

// Нейтральная 404-заглушка (review-фикс P2): рендерится в layout (шапка/i18n сохранены).
// Полноценная страница — позже; здесь честное «страница не найдена» без taboo-формулировок.
export function NotFoundView() {
  const { t } = useTranslation('chrome');
  return (
    <section className="notfound">
      <h2>{t('notfound.title')}</h2>
      <p>{t('notfound.body')}</p>
      <Link to="/">{t('notfound.home')}</Link>
    </section>
  );
}

// errorElement корня (review-фикс P2): нейтральный экран при неожиданной ошибке маршрута.
// Деталь ошибки — только в консоль, пользователю — нейтральный текст (без белого экрана RR на англ.).
export function RouteError() {
  const { t } = useTranslation('chrome');
  const error = useRouteError();
  console.error('Route error:', error);
  return (
    <main>
      <section className="route-error" role="alert">
        <h2>{t('error.title')}</h2>
        <p>{t('error.body')}</p>
        <Link to="/">{t('notfound.home')}</Link>
      </section>
    </main>
  );
}
