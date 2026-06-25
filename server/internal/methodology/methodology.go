// Package methodology — рантайм-загрузчик ИММУТАБЕЛЬНОГО конфига порогов methodology_params (Story 4.1,
// AC1). Источник истины — registry/values/methodology_params.vN.yaml (человек правит ТОЛЬКО там; читается
// В РАНТАЙМЕ, не codegen — принцип AR-12 как у registry). Отдаёт типизированные flags.Params.
//
// НЕ в чистом ядре (импортирует os/yaml — файловый IO), поэтому НЕ нарушает страж чистоты benchmark/flags.
// Честно падает (честность над домыслом): отсутствующий/битый файл, невалидная версия, рассинхрон порогов —
// явная ошибка на старте, а НЕ тихий ноль/дефолт.
package methodology

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/median"
)

// ParamsFile — имя текущей версии конфига в registry/values. Новая версия порогов = НОВЫЙ файл (vN+1).
const ParamsFile = "methodology_params.v1.yaml"

// versionRe — формат methodology_version (AC1): vMAJOR.MINOR (например v1.0).
var versionRe = regexp.MustCompile(`^v\d+\.\d+$`)

// fileShape — форма YAML-источника (registry/values/methodology_params.vN.yaml).
type fileShape struct {
	MethodologyVersion string `yaml:"methodology_version"`
	Median             struct {
		MinSample                 int `yaml:"min_sample"`
		ComparabilityWindowMonths int `yaml:"comparability_window_months"`
	} `yaml:"median"`
	Flag struct {
		PricePerKM struct {
			DeviationFactor float64 `yaml:"deviation_factor"`
		} `yaml:"price_per_km"`
		Monopoly struct {
			ConcentrationShare float64 `yaml:"concentration_share"`
			MinGroupContracts  int     `yaml:"min_group_contracts"`
		} `yaml:"monopoly"`
		SingleParticipant struct {
			Enabled        *bool    `yaml:"enabled"` // *bool: nil = секция/ключ отсутствует → honest-fail (не тихий false)
			ExcludeMethods []string `yaml:"exclude_methods"`
		} `yaml:"single_participant"`
		RNU struct {
			Enabled *bool `yaml:"enabled"` // *bool: nil = секция/ключ отсутствует → honest-fail (не тихий false)
		} `yaml:"rnu"`
	} `yaml:"flag"`
}

// Load читает иммутабельные пороги из <registryRoot>/values/methodology_params.v1.yaml и отдаёт
// типизированные flags.Params. registryRoot — корень registry (например "registry" в проде или
// "../../../registry" в тестах), как у registry.Load.
func Load(registryRoot string) (flags.Params, error) {
	path := filepath.Join(registryRoot, "values", ParamsFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return flags.Params{}, fmt.Errorf("methodology: чтение %s: %w", path, err)
	}
	var f fileShape
	// KnownFields(true): неизвестное/опечатанное имя ключа → ЧЕСТНАЯ ошибка парса (указывает на ключ), а не
	// тихий ноль → вводящая в заблуждение ошибка валидации ниже («рассинхрон min_sample»). Дубли ключей
	// yaml.v3 отвергает и так.
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return flags.Params{}, fmt.Errorf("methodology: парс %s: %w", path, err)
	}

	p := flags.Params{
		MethodologyVersion:         f.MethodologyVersion,
		MinSample:                  f.Median.MinSample,
		ComparabilityWindowMonths:  f.Median.ComparabilityWindowMonths,
		PricePerKMDeviationFactor:  f.Flag.PricePerKM.DeviationFactor,
		MonopolyConcentrationShare: f.Flag.Monopoly.ConcentrationShare,
		MonopolyMinGroupContracts:  f.Flag.Monopoly.MinGroupContracts,

		SingleParticipantEnabled:        f.Flag.SingleParticipant.Enabled != nil && *f.Flag.SingleParticipant.Enabled,
		SingleParticipantExcludeMethods: f.Flag.SingleParticipant.ExcludeMethods,

		RNUEnabled: f.Flag.RNU.Enabled != nil && *f.Flag.RNU.Enabled,
	}
	if err := validate(p); err != nil {
		return flags.Params{}, fmt.Errorf("methodology: %s: %w", path, err)
	}
	// flag.single_participant.enabled обязателен ЯВНО (true/false): отсутствие = honest-fail, чтобы случайное
	// удаление секции не отключало флаг молча (тихий ноль обходит честность, как пропуск ключа glossary).
	if f.Flag.SingleParticipant.Enabled == nil {
		return flags.Params{}, fmt.Errorf("methodology: %s: flag.single_participant.enabled отсутствует (нужен явный true/false)", path)
	}
	// flag.rnu.enabled обязателен ЯВНО (true/false): отсутствие = honest-fail, чтобы случайное удаление секции не
	// отключало флаг РНУ молча (тихий ноль обходит честность, как single_participant).
	if f.Flag.RNU.Enabled == nil {
		return flags.Params{}, fmt.Errorf("methodology: %s: flag.rnu.enabled отсутствует (нужен явный true/false)", path)
	}
	return p, nil
}

// validate — честные инварианты конфига. Пустой/нулевой порог = honest-fail (тихий ноль молча сломал бы
// флаги/медиану). min_sample обязан совпадать с median.MinSample (единый источник; перекрёстный инвариант).
func validate(p flags.Params) error {
	if !versionRe.MatchString(p.MethodologyVersion) {
		return fmt.Errorf("невалидная methodology_version %q (формат ^v\\d+\\.\\d+$)", p.MethodologyVersion)
	}
	if p.MinSample != median.MinSample {
		return fmt.Errorf("min_sample=%d != median.MinSample=%d (рассинхрон single-source ядра и конфига)", p.MinSample, median.MinSample)
	}
	if p.ComparabilityWindowMonths <= 0 {
		return fmt.Errorf("comparability_window_months=%d: ожидалось > 0", p.ComparabilityWindowMonths)
	}
	if p.PricePerKMDeviationFactor <= 0 {
		return fmt.Errorf("price_per_km.deviation_factor=%v: ожидалось > 0", p.PricePerKMDeviationFactor)
	}
	if p.MonopolyConcentrationShare <= 0 || p.MonopolyConcentrationShare > 1 {
		return fmt.Errorf("monopoly.concentration_share=%v: ожидалось в (0, 1]", p.MonopolyConcentrationShare)
	}
	if p.MonopolyMinGroupContracts <= 0 {
		return fmt.Errorf("monopoly.min_group_contracts=%d: ожидалось > 0", p.MonopolyMinGroupContracts)
	}
	// single_participant: пустой способ закупки в exclude_methods = honest-fail (тихая пустая строка молча
	// никогда не совпала бы / маскировала бы опечатку). Пустой список допустим (сигналить любой 1 участник).
	for i, m := range p.SingleParticipantExcludeMethods {
		if strings.TrimSpace(m) == "" {
			return fmt.Errorf("single_participant.exclude_methods[%d]: пустой способ закупки", i)
		}
	}
	return nil
}
