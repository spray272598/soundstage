package domain

import (
	"testing"
	"time"
)

func TestPKSessionStateMachine(t *testing.T) {
	pk := NewPKSession("s1", "roomA", "roomB", "anchorA", "anchorB")
	if pk.Status != PKStatusPending {
		t.Fatalf("expected pending, got %s", pk.Status)
	}

	// Scoring before start must fail.
	if err := pk.AddScore(PKSideA, 100); err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	if err := pk.Accept(); err != nil {
		t.Fatalf("accept failed: %v", err)
	}
	if pk.Status != PKStatusMatched {
		t.Fatalf("expected matched, got %s", pk.Status)
	}

	if err := pk.Start(5 * time.Minute); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if pk.Status != PKStatusOngoing || pk.EndsAt == nil {
		t.Fatalf("expected ongoing with ends_at, got %s ends=%v", pk.Status, pk.EndsAt)
	}

	if err := pk.AddScore(PKSideA, 30); err != nil {
		t.Fatalf("score a failed: %v", err)
	}
	if err := pk.AddScore(PKSideB, 50); err != nil {
		t.Fatalf("score b failed: %v", err)
	}
	if pk.ScoreA != 30 || pk.ScoreB != 50 {
		t.Fatalf("unexpected scores a=%d b=%d", pk.ScoreA, pk.ScoreB)
	}

	// Unknown side is rejected.
	if err := pk.AddScore(PKSide("x"), 1); err != ErrInvalidSide {
		t.Fatalf("expected invalid side, got %v", err)
	}

	if err := pk.Finish(); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	if pk.Status != PKStatusFinished {
		t.Fatalf("expected finished, got %s", pk.Status)
	}
	if pk.Winner != PKWinnerB {
		t.Fatalf("expected winner b, got %s", pk.Winner)
	}

	// Cannot transition after finish.
	if err := pk.Start(time.Minute); err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestPKSessionSideOfRoom(t *testing.T) {
	pk := NewPKSession("s1", "roomA", "roomB", "anchorA", "anchorB")
	side, ok := pk.SideOfRoom("roomA")
	if !ok || side != PKSideA {
		t.Fatalf("expected side a for roomA, got %s ok=%v", side, ok)
	}
	if _, ok := pk.SideOfRoom("roomX"); ok {
		t.Fatalf("expected roomX not a participant")
	}
}
