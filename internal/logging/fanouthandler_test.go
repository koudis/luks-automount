package logging

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type recordingHandler struct {
	records []slog.Record
	level   slog.Level
}

func (h *recordingHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

func newRecord(level slog.Level, msg string) slog.Record {
	return slog.NewRecord(time.Time{}, level, msg, 0)
}

func TestFanoutHandler_DeliversToBothHandlers(t *testing.T) {
	a := &recordingHandler{level: slog.LevelDebug}
	b := &recordingHandler{level: slog.LevelDebug}
	f := newFanoutHandler(a, b)

	_ = f.Handle(context.Background(), newRecord(slog.LevelInfo, "hello"))

	if len(a.records) != 1 {
		t.Errorf("handler a: expected 1 record, got %d", len(a.records))
	}
	if len(b.records) != 1 {
		t.Errorf("handler b: expected 1 record, got %d", len(b.records))
	}
}

func TestFanoutHandler_Enabled_AnyTrue(t *testing.T) {
	low := &recordingHandler{level: slog.LevelDebug}
	high := &recordingHandler{level: slog.LevelError}
	f := newFanoutHandler(low, high)

	if !f.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Enabled=true when at least one handler is enabled")
	}
}

func TestFanoutHandler_Enabled_AllFalse(t *testing.T) {
	h1 := &recordingHandler{level: slog.LevelError}
	h2 := &recordingHandler{level: slog.LevelError}
	f := newFanoutHandler(h1, h2)

	if f.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Enabled=false when no handler is enabled at that level")
	}
}

func TestFanoutHandler_SkipsDisabledHandler(t *testing.T) {
	low := &recordingHandler{level: slog.LevelDebug}
	high := &recordingHandler{level: slog.LevelError}
	f := newFanoutHandler(low, high)

	_ = f.Handle(context.Background(), newRecord(slog.LevelInfo, "msg"))

	if len(low.records) != 1 {
		t.Errorf("low handler expected 1 record, got %d", len(low.records))
	}
	if len(high.records) != 0 {
		t.Errorf("high handler expected 0 records, got %d", len(high.records))
	}
}

