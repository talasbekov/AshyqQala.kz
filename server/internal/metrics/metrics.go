package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler — обработчик /metrics (Prometheus-экспозиция). В S-0 отдаёт стандартные go-метрики;
// доменные счётчики SM-C1/SM-C2 регистрируются здесь по мере появления (Epic 4, в домене).
func Handler() http.Handler {
	return promhttp.Handler()
}
