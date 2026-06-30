package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestAuditLogger_WritesStructuredEvent(t *testing.T) {
	var buf bytes.Buffer
	audit := NewAuditLogger(&buf)

	fixedTime := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	audit.Log(context.Background(), AuditEvent{
		Time:    fixedTime,
		Actor:   "user:alice",
		Action:  "case.viewed",
		Target:  "case:1234",
		Outcome: "success",
	})

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if record["actor"] != "user:alice" {
		t.Errorf("actor = %v, want user:alice", record["actor"])
	}
	if record["action"] != "case.viewed" {
		t.Errorf("action = %v, want case.viewed", record["action"])
	}
	if record["target"] != "case:1234" {
		t.Errorf("target = %v, want case:1234", record["target"])
	}
	if record["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", record["outcome"])
	}
}

func TestAuditLogger_FillsZeroTime(t *testing.T) {
	var buf bytes.Buffer
	audit := NewAuditLogger(&buf)

	before := time.Now().UTC()
	audit.Log(context.Background(), AuditEvent{Actor: "system", Action: "startup", Outcome: "success"})
	after := time.Now().UTC()

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	stamped, err := time.Parse(time.RFC3339Nano, record["time"].(string))
	if err != nil {
		t.Fatalf("could not parse stamped time: %v", err)
	}
	if stamped.Before(before) || stamped.After(after) {
		t.Errorf("stamped time %v not within [%v, %v]", stamped, before, after)
	}
}

func TestAuditLogger_SeparateFromAppLog(t *testing.T) {
	var appBuf, auditBuf bytes.Buffer

	appLogger := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&appBuf))
	auditLogger := NewAuditLogger(&auditBuf)

	appLogger.Info(context.Background(), "application event")
	auditLogger.Log(context.Background(), AuditEvent{Actor: "user:bob", Action: "case.exported", Outcome: "success"})

	if appBuf.Len() == 0 {
		t.Fatal("expected application log output")
	}
	if auditBuf.Len() == 0 {
		t.Fatal("expected audit log output")
	}

	// Each sink must contain only its own record - the whole point of
	// a separate channel is that they never commingle.
	var appRecord, auditRecord map[string]any
	if err := json.Unmarshal(appBuf.Bytes(), &appRecord); err != nil {
		t.Fatalf("invalid app log JSON: %v", err)
	}
	if err := json.Unmarshal(auditBuf.Bytes(), &auditRecord); err != nil {
		t.Fatalf("invalid audit log JSON: %v", err)
	}
	if _, ok := appRecord["actor"]; ok {
		t.Error("application log unexpectedly contains an audit field")
	}
	if _, ok := auditRecord["msg"]; ok && auditRecord["msg"] != "audit_event" {
		t.Error("audit log unexpectedly contains an unrelated application message")
	}
}
