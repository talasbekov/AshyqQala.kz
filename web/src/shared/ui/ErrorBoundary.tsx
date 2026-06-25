import { Component, type ErrorInfo, type ReactNode } from 'react';

// ErrorBoundary — честный fallback вместо белого экрана при рендер-throw (Story 5.3, закрывает defer ревью 5-1).
// React error boundary обязан быть class-компонентом. Ловит throw из строгих парсеров (formatMoney/formatDate)
// и любой рендер-ошибки поддерева → показывает переданный fallback, НЕ роняет всю страницу.
interface Props {
  fallback: ReactNode;
  children: ReactNode;
}
interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // Не глотаем молча — в консоль для диагностики (продакшн-логирование — отдельная app-wide тема).
    console.error('ErrorBoundary поймал рендер-ошибку', error, info.componentStack);
  }

  render(): ReactNode {
    return this.state.hasError ? this.props.fallback : this.props.children;
  }
}
