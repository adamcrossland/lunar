package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// captureOutput redirects outputWriter to a buffer for the duration of f,
// then restores it and returns what was written.
func captureOutput(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := outputWriter
	outputWriter = &buf
	t.Cleanup(func() { outputWriter = prev })
	f()
	return buf.String()
}

// ── detectEnvelope ────────────────────────────────────────────────────────────

func TestDetectEnvelope_List(t *testing.T) {
	obj := map[string]any{
		"functions":  []any{"a", "b"},
		"pagination": map[string]any{"total": 2.0, "limit": 20.0, "offset": 0.0},
	}
	items, pag, ok := detectEnvelope(obj)
	if !ok {
		t.Fatal("expected envelope to be detected")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if pag == nil {
		t.Fatal("expected pagination")
	}
}

func TestDetectEnvelope_NullArray(t *testing.T) {
	// Null list field (empty result set from server)
	obj := map[string]any{
		"executions": nil,
		"pagination": map[string]any{"total": 0.0, "limit": 20.0, "offset": 0.0},
	}
	items, _, ok := detectEnvelope(obj)
	if !ok {
		t.Fatal("expected null array to be detected as envelope")
	}
	if items != nil {
		t.Fatal("expected items to be nil for null array")
	}
}

func TestDetectEnvelope_NotEnvelope_MultipleArrays(t *testing.T) {
	obj := map[string]any{
		"a": []any{1},
		"b": []any{2},
	}
	_, _, ok := detectEnvelope(obj)
	if ok {
		t.Fatal("multiple arrays should not be detected as envelope")
	}
}

func TestDetectEnvelope_NotEnvelope_ScalarField(t *testing.T) {
	obj := map[string]any{
		"items": []any{1},
		"name":  "foo", // non-pagination scalar → not an envelope
	}
	_, _, ok := detectEnvelope(obj)
	if ok {
		t.Fatal("object with extra scalar field should not be envelope")
	}
}

func TestDetectEnvelope_EmptyObject(t *testing.T) {
	_, _, ok := detectEnvelope(map[string]any{})
	if ok {
		t.Fatal("empty object should not be detected as envelope")
	}
}

// ── formatValue ───────────────────────────────────────────────────────────────

func TestFormatValue_String(t *testing.T) {
	if got := formatValue("name", "hello"); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestFormatValue_StringTruncation(t *testing.T) {
	long := strings.Repeat("x", 80)
	got := formatValue("name", long)
	if len(got) > 72 {
		t.Errorf("expected truncation to 72 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected truncated string to end with ...")
	}
}

func TestFormatValue_Bool(t *testing.T) {
	if got := formatValue("disabled", true); got != "true" {
		t.Errorf("got %q", got)
	}
	if got := formatValue("disabled", false); got != "false" {
		t.Errorf("got %q", got)
	}
}

func TestFormatValue_Integer(t *testing.T) {
	if got := formatValue("version", 3.0); got != "3" {
		t.Errorf("got %q, want \"3\"", got)
	}
}

func TestFormatValue_Float(t *testing.T) {
	if got := formatValue("ratio", 1.5); got != "1.5" {
		t.Errorf("got %q, want \"1.5\"", got)
	}
}

func TestFormatValue_Nil(t *testing.T) {
	if got := formatValue("any", nil); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestFormatValue_Timestamp_At(t *testing.T) {
	ts := float64(time.Date(2026, 3, 4, 18, 18, 0, 0, time.UTC).Unix())
	got := formatValue("created_at", ts)
	if got == "" || got == "0" {
		t.Errorf("timestamp not formatted: %q", got)
	}
	// Should contain the date portion
	if !strings.Contains(got, "2026-03-04") {
		t.Errorf("expected date 2026-03-04 in %q", got)
	}
}

func TestFormatValue_Timestamp_LastUsed(t *testing.T) {
	ts := float64(time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC).Unix())
	got := formatValue("last_used", ts)
	if !strings.Contains(got, "2026-04-18") {
		t.Errorf("expected date 2026-04-18 in %q", got)
	}
}

func TestFormatValue_ZeroTimestamp(t *testing.T) {
	// Zero/null timestamp should not be formatted as a date
	got := formatValue("created_at", 0.0)
	if got != "0" {
		t.Errorf("zero timestamp should render as '0', got %q", got)
	}
}

// ── isTimestampKey ────────────────────────────────────────────────────────────

func TestIsTimestampKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"created_at", true},
		{"updated_at", true},
		{"last_used", true},
		{"timestamp", true},
		{"name", false},
		{"status", false},
		{"at", false}, // doesn't end in _at
	}
	for _, c := range cases {
		if got := isTimestampKey(c.key); got != c.want {
			t.Errorf("isTimestampKey(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

// ── isSimpleValue ─────────────────────────────────────────────────────────────

func TestIsSimpleValue(t *testing.T) {
	if !isSimpleValue("hello") {
		t.Error("string should be simple")
	}
	if !isSimpleValue(true) {
		t.Error("bool should be simple")
	}
	if !isSimpleValue(42.0) {
		t.Error("float64 should be simple")
	}
	if !isSimpleValue(nil) {
		t.Error("nil should be simple")
	}
	if isSimpleValue([]any{1}) {
		t.Error("slice should not be simple")
	}
	if isSimpleValue(map[string]any{"k": "v"}) {
		t.Error("map should not be simple")
	}
}

// ── orderedKeys ───────────────────────────────────────────────────────────────

func TestOrderedKeys_PreferredFirst(t *testing.T) {
	keys := map[string]struct{}{
		"description": {},
		"name":        {},
		"id":          {},
		"created_at":  {},
	}
	got := orderedKeys(keys)
	if got[0] != "id" {
		t.Errorf("expected id first, got %v", got)
	}
	if got[1] != "name" {
		t.Errorf("expected name second, got %v", got)
	}
	// description is not in preferredColumns, so it comes after
	last := got[len(got)-1]
	if last != "description" && last != "created_at" {
		// created_at is preferred, description is not — description should be last
		t.Errorf("unexpected last element: %v", got)
	}
}

func TestOrderedKeys_AlphabeticFallback(t *testing.T) {
	keys := map[string]struct{}{
		"zebra": {},
		"alpha": {},
		"moon":  {},
	}
	got := orderedKeys(keys)
	if got[0] != "alpha" || got[1] != "moon" || got[2] != "zebra" {
		t.Errorf("expected alphabetic order, got %v", got)
	}
}

// ── rendering output ─────────────────────────────────────────────────────────

func TestPrintRawJSON_Valid(t *testing.T) {
	out := captureOutput(t, func() {
		_ = printRawJSON([]byte(`{"name":"hello","version":3}`))
	})
	if !strings.Contains(out, `"name"`) {
		t.Errorf("expected key in output: %q", out)
	}
	if !strings.Contains(out, `"hello"`) {
		t.Errorf("expected value in output: %q", out)
	}
}

func TestPrintRawJSON_Invalid(t *testing.T) {
	out := captureOutput(t, func() {
		_ = printRawJSON([]byte(`not json`))
	})
	if !strings.Contains(out, "not json") {
		t.Errorf("expected raw fallback in output: %q", out)
	}
}

func TestPrintPretty_EmptyArray(t *testing.T) {
	out := captureOutput(t, func() {
		_ = printPretty([]byte(`[]`))
	})
	if !strings.Contains(out, "no results") {
		t.Errorf("expected '(no results)', got: %q", out)
	}
}

func TestPrintPretty_Envelope_EmptyList(t *testing.T) {
	out := captureOutput(t, func() {
		_ = printPretty([]byte(`{"executions":null,"pagination":{"total":0,"limit":20,"offset":0}}`))
	})
	if !strings.Contains(out, "no results") {
		t.Errorf("expected '(no results)', got: %q", out)
	}
	if strings.Contains(out, "1–0") {
		t.Errorf("empty pagination footer should not render an invalid range: %q", out)
	}
	if !strings.Contains(out, "showing 0 of 0") {
		t.Errorf("expected empty pagination footer in output: %q", out)
	}
}

func TestPrintAPIResponse_ReturnsErrorOnHTTPFailure(t *testing.T) {
	err := printAPIResponse(401, []byte(`{"error":"unauthorized"}`))
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected status code in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestPrintPretty_Envelope_WithItems(t *testing.T) {
	payload := `{"functions":[{"id":"abc123","name":"hello","disabled":false}],"pagination":{"total":1,"limit":20,"offset":0}}`
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	if !strings.Contains(out, "abc123") {
		t.Errorf("expected id in output: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected name in output: %q", out)
	}
	if !strings.Contains(out, "showing") {
		t.Errorf("expected pagination footer: %q", out)
	}
}

func TestPrintPretty_Object(t *testing.T) {
	payload := `{"id":"abc","name":"myfunc","disabled":false,"created_at":1771150328}`
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	if !strings.Contains(out, "abc") {
		t.Errorf("expected id in output: %q", out)
	}
	if !strings.Contains(out, "myfunc") {
		t.Errorf("expected name in output: %q", out)
	}
	// Timestamp should be formatted as a date, not raw number
	if strings.Contains(out, "1771150328") {
		t.Errorf("raw timestamp should be formatted: %q", out)
	}
}

func TestPrintPretty_Object_CodeHiddenByDefault(t *testing.T) {
	payload := `{"id":"abc","code":"function handler() end"}`
	showCode = false
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	if strings.Contains(out, "function handler()") {
		t.Errorf("code should be hidden by default: %q", out)
	}
	if !strings.Contains(out, "--show-code") {
		t.Errorf("expected hint about --show-code: %q", out)
	}
}

func TestPrintPretty_Object_CodeShownWithFlag(t *testing.T) {
	payload := `{"id":"abc","code":"function handler() end"}`
	showCode = true
	t.Cleanup(func() { showCode = false })
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	if !strings.Contains(out, "function handler()") {
		t.Errorf("code should be visible with --show-code: %q", out)
	}
}

func TestPrintPretty_Object_EnvVarsTable(t *testing.T) {
	payload := `{"id":"abc","env_vars":{"API_KEY":"secret","DEBUG":"true"}}`
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	if !strings.Contains(out, "env_vars") {
		t.Errorf("expected env_vars section: %q", out)
	}
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("expected API_KEY in env table: %q", out)
	}
	if !strings.Contains(out, "secret") {
		t.Errorf("expected value in env table: %q", out)
	}
}

func TestPrintPretty_Object_EmptyEnvVars(t *testing.T) {
	payload := `{"id":"abc","env_vars":{}}`
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	if !strings.Contains(out, "empty") {
		t.Errorf("expected '(empty)' for empty map: %q", out)
	}
}

func TestPrintPretty_List_SkipsComplexFields(t *testing.T) {
	payload := `[{"id":"abc","name":"fn","active_version":{"id":"v1","code":"..."}}]`
	out := captureOutput(t, func() {
		_ = printPretty([]byte(payload))
	})
	// id and name should appear
	if !strings.Contains(out, "abc") {
		t.Errorf("expected id in list output: %q", out)
	}
	// active_version is in skipInListView, should not appear
	if strings.Contains(out, "active_version") {
		t.Errorf("active_version should be skipped in list view: %q", out)
	}
}
