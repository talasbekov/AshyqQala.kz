package methodology_test

import (
	"os"
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/median"
	"ashyqqala/server/internal/methodology"
)

// repoRoot — корень репо относительно каталога пакета (server/internal/methodology).
const repoRoot = "../../../"

// TestLoad_Defaults — реальный registry/values/methodology_params.v1.yaml: все 5 порогов + версия читаются
// и РАВНЫ дефолтам data-model §2. Пин golden-литералами (drift порога → красный тест).
func TestLoad_Defaults(t *testing.T) {
	p, err := methodology.Load(filepath.Join(repoRoot, "registry"))
	if err != nil {
		t.Fatalf("Load(registry): %v", err)
	}
	if p.MethodologyVersion != "v1.0" {
		t.Errorf("methodology_version = %q, ожидалось v1.0", p.MethodologyVersion)
	}
	if p.MinSample != 5 {
		t.Errorf("min_sample = %d, ожидалось 5", p.MinSample)
	}
	if p.ComparabilityWindowMonths != 24 {
		t.Errorf("comparability_window_months = %d, ожидалось 24", p.ComparabilityWindowMonths)
	}
	if p.PricePerKMDeviationFactor != 1.5 {
		t.Errorf("deviation_factor = %v, ожидалось 1.5", p.PricePerKMDeviationFactor)
	}
	if p.MonopolyConcentrationShare != 0.5 {
		t.Errorf("concentration_share = %v, ожидалось 0.5", p.MonopolyConcentrationShare)
	}
	if p.MonopolyMinGroupContracts != 5 {
		t.Errorf("min_group_contracts = %d, ожидалось 5", p.MonopolyMinGroupContracts)
	}
	if !p.SingleParticipantEnabled {
		t.Errorf("single_participant.enabled = %v, ожидалось true", p.SingleParticipantEnabled)
	}
	if len(p.SingleParticipantExcludeMethods) != 1 || p.SingleParticipantExcludeMethods[0] != "из_одного_источника" {
		t.Errorf("single_participant.exclude_methods = %v, ожидалось [из_одного_источника]", p.SingleParticipantExcludeMethods)
	}
	if !p.RNUEnabled {
		t.Errorf("rnu.enabled = %v, ожидалось true", p.RNUEnabled)
	}
}

// TestLoad_MinSampleMatchesCore — single-source: порог выборки конфига == median.MinSample (чистое ядро).
func TestLoad_MinSampleMatchesCore(t *testing.T) {
	p, err := methodology.Load(filepath.Join(repoRoot, "registry"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.MinSample != median.MinSample {
		t.Fatalf("min_sample конфига=%d != median.MinSample=%d", p.MinSample, median.MinSample)
	}
}

// TestLoad_MissingFile_HonestError — отсутствующий файл → честная ошибка (не тихий дефолт).
func TestLoad_MissingFile_HonestError(t *testing.T) {
	if _, err := methodology.Load(t.TempDir()); err == nil {
		t.Fatal("ожидалась честная ошибка на отсутствующем methodology_params.v1.yaml, got nil")
	}
}

// writeParams — пишет временный registry/values/methodology_params.v1.yaml с заданным телом, возвращает root.
func writeParams(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	vals := filepath.Join(root, "values")
	if err := os.MkdirAll(vals, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vals, methodology.ParamsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const validBody = `methodology_version: v1.0
median:
  min_sample: 5
  comparability_window_months: 24
flag:
  single_participant:
    enabled: true
    exclude_methods:
      - из_одного_источника
  price_per_km:
    deviation_factor: 1.5
  monopoly:
    concentration_share: 0.5
    min_group_contracts: 5
  rnu:
    enabled: true
`

// TestLoad_ValidTmp_OK — позитивный контроль: корректное тело грузится без ошибки (страж не «всегда красный»).
func TestLoad_ValidTmp_OK(t *testing.T) {
	if _, err := methodology.Load(writeParams(t, validBody)); err != nil {
		t.Fatalf("корректный конфиг должен грузиться, got %v", err)
	}
}

// TestLoad_HonestErrors — negative-control: загрузчик ОБЯЗАН покраснеть на каждом классе невалидности
// (невалидная версия, рассинхрон min_sample с ядром, нулевой/выходящий за диапазон порог, битый YAML).
// См. [[guards-must-prove-red]]: страж доказывает, что умеет краснеть.
func TestLoad_HonestErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"невалидная версия", `methodology_version: 1.0
median: {min_sample: 5, comparability_window_months: 24}
flag: {price_per_km: {deviation_factor: 1.5}, monopoly: {concentration_share: 0.5, min_group_contracts: 5}}`},
		{"рассинхрон min_sample с ядром", `methodology_version: v1.0
median: {min_sample: 4, comparability_window_months: 24}
flag: {price_per_km: {deviation_factor: 1.5}, monopoly: {concentration_share: 0.5, min_group_contracts: 5}}`},
		{"нулевое окно", `methodology_version: v1.0
median: {min_sample: 5, comparability_window_months: 0}
flag: {price_per_km: {deviation_factor: 1.5}, monopoly: {concentration_share: 0.5, min_group_contracts: 5}}`},
		{"нулевой deviation_factor", `methodology_version: v1.0
median: {min_sample: 5, comparability_window_months: 24}
flag: {price_per_km: {deviation_factor: 0}, monopoly: {concentration_share: 0.5, min_group_contracts: 5}}`},
		{"share вне (0,1]", `methodology_version: v1.0
median: {min_sample: 5, comparability_window_months: 24}
flag: {price_per_km: {deviation_factor: 1.5}, monopoly: {concentration_share: 1.5, min_group_contracts: 5}}`},
		{"битый YAML", `methodology_version: v1.0
median: [this is not a map`},
		{"single_participant.enabled отсутствует", `methodology_version: v1.0
median: {min_sample: 5, comparability_window_months: 24}
flag: {price_per_km: {deviation_factor: 1.5}, monopoly: {concentration_share: 0.5, min_group_contracts: 5}, rnu: {enabled: true}}`},
		{"rnu.enabled отсутствует", `methodology_version: v1.0
median: {min_sample: 5, comparability_window_months: 24}
flag: {price_per_km: {deviation_factor: 1.5}, monopoly: {concentration_share: 0.5, min_group_contracts: 5}, single_participant: {enabled: true}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := methodology.Load(writeParams(t, c.body)); err == nil {
				t.Errorf("ожидалась честная ошибка (%s), got nil", c.name)
			}
		})
	}
}
