// Package apierr — закрытый каталог error-кодов (UPPER_SNAKE) + честный JSON-ответ.
// Каталог зеркалится в OpenAPI responses. Расширяется по мере появления эндпоинтов.
package apierr

import (
	"encoding/json"
	"net/http"
)

type Code string

const (
	CodeNotFound         Code = "NOT_FOUND"
	CodeValidationFailed Code = "VALIDATION_FAILED"
	CodeInvalidCursor    Code = "INVALID_CURSOR" // задел для keyset-пагинации (Epic 6)
	CodeRateLimited      Code = "RATE_LIMITED"   // задел
	CodeInternal         Code = "INTERNAL"
)

// Body — форма тела ошибки на проводе (snake_case).
type Body struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

// Write пишет честный error-ответ: {"error":{"code":...,"message":...}}.
func Write(w http.ResponseWriter, status int, code Code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]Body{"error": {Code: code, Message: msg}})
}
