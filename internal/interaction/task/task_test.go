package task

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPersistDanmakuRoundTrip ensures the wire format the enqueuer produces is
// exactly what the asynq worker decodes. The worker unmarshals into this same
// struct with json.Unmarshal, so the keys must be snake_case and the struct
// tags must match. A mismatch silently empties every field (see prior regression).
func TestPersistDanmakuRoundTrip(t *testing.T) {
	orig := PersistDanmakuPayload{
		ID:        "dm-1",
		RoomID:    "room-7",
		UserID:    "user-9",
		Content:   "hello",
		Status:    "approved",
		CreatedAt: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Enforce the snake_case wire contract the worker expects.
	for _, key := range []string{"id", "room_id", "user_id", "content", "status", "created_at"} {
		if !containsKey(t, data, key) {
			t.Errorf("payload missing key %q: %s", key, data)
		}
	}

	var got PersistDanmakuPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", got, orig)
	}
}

// TestSettleGiftRoundTrip mirrors the above for gift settlement payloads.
func TestSettleGiftRoundTrip(t *testing.T) {
	orig := SettleGiftPayload{OrderID: "order-42"}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !containsKey(t, data, "order_id") {
		t.Errorf("payload missing key order_id: %s", data)
	}

	var got SettleGiftPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch: got=%+v want=%+v", got, orig)
	}
}

func containsKey(t *testing.T, data []byte, key string) bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("inspect payload: %v", err)
	}
	_, ok := m[key]
	return ok
}
