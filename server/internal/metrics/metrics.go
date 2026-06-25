package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SM-C1 (контр-метрика точности флагов, prd#SM-C1; architecture:62-63,843) измеряется DB-derived из flag_disputes
// (источник истины), а НЕ императивным in-process счётчиком (решение ревью 4.6): значения восстанавливаются из БД
// на КАЖДОМ scrape /metrics — restart-safe (рестарт процесса не теряет историю), race-free (нет read-modify-write)
// и идемпотентны (повтор/откат статуса не двоит и не раздувает). reviewed = прошедшие ручную верификацию
// (confirmed|withdrawn) — знаменатель; incorrect = признанные некорректными (withdrawn) — числитель. confirmed =
// флаг подтверждён валидным сигналом ⇒ НЕ incorrect. SM-C1 = incorrect/reviewed (вычисляет дашборд).

// SMC1Counts — мгновенный срез SM-C1 из flag_disputes.
type SMC1Counts struct {
	Reviewed  float64 // confirmed + withdrawn (знаменатель)
	Incorrect float64 // withdrawn (числитель)
}

// SMC1Source возвращает текущий срез из БД (flag_disputes). Вызывается на КАЖДОМ scrape /metrics.
type SMC1Source func(ctx context.Context) (SMC1Counts, error)

var (
	smc1ReviewedDesc = prometheus.NewDesc(
		"ashyqqala_flags_reviewed_total",
		"Флаги, прошедшие ручную верификацию (confirmed|withdrawn) — знаменатель SM-C1. DB-derived из flag_disputes.",
		nil, nil,
	)
	smc1IncorrectDesc = prometheus.NewDesc(
		"ashyqqala_flags_incorrect_total",
		"Флаги, признанные некорректными при ручной проверке (withdrawn) — числитель SM-C1. DB-derived из flag_disputes.",
		nil, nil,
	)
)

// smc1Collector — prometheus.Collector, считывающий SM-C1 из БД на каждом scrape (не хранит состояние в процессе).
type smc1Collector struct {
	src SMC1Source
	log *slog.Logger
}

func (c smc1Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- smc1ReviewedDesc
	ch <- smc1IncorrectDesc
}

func (c smc1Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	counts, err := c.src(ctx)
	if err != nil {
		// Честность над домыслом: при ошибке источника НЕ отдаём ложный 0 (дашборд посчитал бы SM-C1 = 0/0).
		// Пропускаем метрики этого scrape (Prometheus покажет staleness), логируем; остальной /metrics жив.
		if c.log != nil {
			c.log.Error("smc1_collect_failed", "error", err.Error())
		}
		return
	}
	ch <- prometheus.MustNewConstMetric(smc1ReviewedDesc, prometheus.CounterValue, counts.Reviewed)
	ch <- prometheus.MustNewConstMetric(smc1IncorrectDesc, prometheus.CounterValue, counts.Incorrect)
}

// RegisterSMC1 регистрирует DB-derived коллектор SM-C1 в default-registry (экспонируется через Handler()).
// Вызывать ОДИН раз на старте (prometheus.MustRegister паникует на дубль). src читает flag_disputes.
func RegisterSMC1(src SMC1Source, log *slog.Logger) {
	prometheus.MustRegister(smc1Collector{src: src, log: log})
}

// Handler — обработчик /metrics (Prometheus-экспозиция): стандартные go-метрики + DB-derived SM-C1 (если
// зарегистрирован через RegisterSMC1).
func Handler() http.Handler {
	return promhttp.Handler()
}
