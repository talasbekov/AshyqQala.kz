package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
	"ashyqqala/server/internal/store/gen"
)

func evidenceRenderer(t *testing.T) render.Renderer {
	t.Helper()
	reg, err := registry.Load("../../../registry")
	if err != nil {
		t.Fatalf("registry load: %v", err)
	}
	return render.Renderer{Reg: reg}
}

func newEvidenceRouter(store ContractStore, rd render.Renderer) http.Handler {
	r := chi.NewRouter()
	h := EvidenceExportHandler{Store: store, Renderer: rd}
	r.Get("/api/contracts/{goszakup_id}/flags/{flag_type}/evidence.json", h.GetJSON)
	r.Get("/api/contracts/{goszakup_id}/flags/{flag_type}/evidence.txt", h.GetText)
	return r
}

func raisedContractFlag(flagType, ver, ev string) gen.RiskFlag {
	return gen.RiskFlag{
		FlagType:           flagType,
		SubjectType:        "contract",
		ContractID:         pgtype.Int8{Int64: 1, Valid: true},
		IsActive:           true,
		Evidence:           []byte(ev),
		MethodologyVersion: ver,
	}
}

// TestEvidenceExport_RaisedFlag_JSONAndText — AR-29/SM-5: raised-флаг → канонический JSON (валиден по OpenAPI
// EvidenceExport; subject_ref = публичный goszakup-id, БЕЗ внутреннего bigint) + печатный нейтральный текст
// (рамка из registry + факты, без оценочных/taboo слов; negative-control taboo).
func TestEvidenceExport_RaisedFlag_JSONAndText(t *testing.T) {
	store := mockStore{
		c:     sampleContract(), // GoszakupContractID = DEMO-0001
		flags: []gen.RiskFlag{raisedContractFlag("price_per_km", "v1.0", `{"price_per_km":2000000,"median":1000000}`)},
	}
	srv := httptest.NewServer(newEvidenceRouter(store, evidenceRenderer(t)))
	defer srv.Close()

	// --- JSON ---
	resp, err := http.Get(srv.URL + "/api/contracts/DEMO-0001/flags/price_per_km/evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("json status=%d; want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type=%q; want application/json", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "EvidenceExport").VisitJSON(body); err != nil {
		t.Fatalf("экспорт не валиден по OpenAPI EvidenceExport: %v", err)
	}
	if body["subject_ref"] != "DEMO-0001" {
		t.Errorf("subject_ref должен быть публичным goszakup-id, got %v", body["subject_ref"])
	}
	if _, leak := body["subject_id"]; leak {
		t.Errorf("внутренний subject_id НЕ должен утекать в экспорт: %v", body)
	}
	if body["evidence"] == nil {
		t.Errorf("evidence не экспортирован: %v", body)
	}

	// --- Печатный текст (ru-рамка) ---
	respT, err := http.Get(srv.URL + "/api/contracts/DEMO-0001/flags/price_per_km/evidence.txt?lang=ru")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = respT.Body.Close() }()
	if respT.StatusCode != http.StatusOK {
		t.Fatalf("text status=%d; want 200", respT.StatusCode)
	}
	if ct := respT.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type=%q; want text/plain", ct)
	}
	tb, _ := io.ReadAll(respT.Body)
	text := string(tb)
	if !strings.Contains(text, "сигнал, требующий проверки") {
		t.Errorf("печатный без нейтральной рамки (ru): %q", text)
	}
	for _, fact := range []string{"price_per_km", "v1.0", "DEMO-0001"} {
		if !strings.Contains(text, fact) {
			t.Errorf("печатный не содержит факт %q: %q", fact, text)
		}
	}
	for _, taboo := range []string{"нарушение", "коррупц", "виновен", "преступл"} {
		if strings.Contains(strings.ToLower(text), taboo) {
			t.Errorf("negative-control: печатный содержит оценочное слово %q: %q", taboo, text)
		}
	}

	// kk-локаль: нейтральная kk-рамка (дефолт без ?lang тоже KK, NFR-6), без kk-taboo.
	respKk, err := http.Get(srv.URL + "/api/contracts/DEMO-0001/flags/price_per_km/evidence.txt?lang=kk")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = respKk.Body.Close() }()
	kb, _ := io.ReadAll(respKk.Body)
	kkText := string(kb)
	if !strings.Contains(kkText, "тексеруді талап ететін сигнал") {
		t.Errorf("печатный без нейтральной рамки (kk): %q", kkText)
	}
	for _, taboo := range []string{"бұзушылық", "сыбайлас жемқорлық", "кінәлі"} {
		if strings.Contains(strings.ToLower(kkText), taboo) {
			t.Errorf("negative-control (kk): печатный содержит оценочное слово %q: %q", taboo, kkText)
		}
	}
}

// TestEvidenceExport_NonRaised_404 — флаг НЕ raised (нет строки → insufficient) → 404: цитировать нечего,
// НЕ фабрикуем пустой документ (§7.4 честность над домыслом).
func TestEvidenceExport_NonRaised_404(t *testing.T) {
	store := mockStore{c: sampleContract(), flags: nil} // нет строк → price_per_km insufficient_data
	srv := httptest.NewServer(newEvidenceRouter(store, evidenceRenderer(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/DEMO-0001/flags/price_per_km/evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("не-raised флаг → status=%d; want 404", resp.StatusCode)
	}
}

// TestEvidenceExport_UnknownContract_404 — контракта нет → 404 (не выдуманный экспорт).
func TestEvidenceExport_UnknownContract_404(t *testing.T) {
	store := mockStore{err: pgx.ErrNoRows}
	srv := httptest.NewServer(newEvidenceRouter(store, evidenceRenderer(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/NOPE/flags/price_per_km/evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("неизвестный контракт → status=%d; want 404", resp.StatusCode)
	}
}
