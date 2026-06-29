package orgnorm_test

import (
	"context"
	"testing"

	"ashyqqala/server/internal/ingest/orgnorm"
)

// TestApply_NilPoolHonestError — Apply(nil pool) → честная ошибка, не nil-паника. DB-free (ошибка до Begin),
// поэтому это обычный unit-тест без build-tag integration.
func TestApply_NilPoolHonestError(t *testing.T) {
	if err := orgnorm.Apply(context.Background(), nil, orgnorm.Plan{}); err == nil {
		t.Fatal("ожидалась честная ошибка при nil pool, got nil")
	}
}
