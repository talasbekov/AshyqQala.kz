package httpapi

import (
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/registry"
)

// Field — честный конверт значения на проводе: {value, state}.
// value:null ⇒ честное состояние "no_data" (НИКОГДА 0 / пустая строка). State — типизированный
// закрытый value_state-enum из registry (Story 1.4); на проводе остаётся lower_snake-строкой.
type Field[T any] struct {
	Value *T                  `json:"value"`
	State registry.ValueState `json:"state"`
}

func okField[T any](v T) Field[T] { return Field[T]{Value: &v, State: registry.StateOK} }
func noData[T any]() Field[T]     { return Field[T]{State: registry.StateNoData} }

func fromText(t pgtype.Text) Field[string] {
	if !t.Valid {
		return noData[string]()
	}
	return okField(t.String)
}

// fromInt8String — деньги/целые: bigint → СТРОКА (wire-конвенция, без float).
func fromInt8String(n pgtype.Int8) Field[string] {
	if !n.Valid {
		return noData[string]()
	}
	return okField(strconv.FormatInt(n.Int64, 10))
}

// fromDate — дата-без-времени → YYYY-MM-DD.
func fromDate(d pgtype.Date) Field[string] {
	if !d.Valid {
		return noData[string]()
	}
	return okField(d.Time.Format("2006-01-02"))
}

// fromTimestamptz — timestamptz → ISO8601 `Z`. NULL → no_data (НЕ фабрикуем "0001-01-01").
func fromTimestamptz(t pgtype.Timestamptz) Field[string] {
	if !t.Valid {
		return noData[string]()
	}
	return okField(t.Time.UTC().Format(time.RFC3339))
}
