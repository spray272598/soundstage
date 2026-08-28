package domain

import (
	"time"
)

// PKStatus is the lifecycle state of a cross-room PK battle.
type PKStatus string

const (
	// PKStatusPending means a room challenged another and is awaiting accept.
	PKStatusPending PKStatus = "pending"
	// PKStatusMatched means the opponent accepted; awaiting start.
	PKStatusMatched PKStatus = "matched"
	// PKStatusOngoing means the battle is live with a running countdown.
	PKStatusOngoing PKStatus = "ongoing"
	// PKStatusFinished means the battle ended and the winner is decided.
	PKStatusFinished PKStatus = "finished"
)

// PKWinner is the outcome of a finished PK.
type PKWinner string

const (
	// PKWinnerA means room A won.
	PKWinnerA PKWinner = "a"
	// PKWinnerB means room B won.
	PKWinnerB PKWinner = "b"
	// PKWinnerDraw means a tie.
	PKWinnerDraw PKWinner = "draw"
	// PKWinnerNone means the battle has not finished.
	PKWinnerNone PKWinner = ""
)

// PKSide identifies which room a score delta belongs to.
type PKSide string

const (
	// PKSideA is the challenging room.
	PKSideA PKSide = "a"
	// PKSideB is the challenged room.
	PKSideB PKSide = "b"
)

// PKSession models a cross-room PK (对战) battle. Both rooms' audiences see the
// same battle; each room's gifts during the battle feed that room's score.
type PKSession struct {
	ID         string
	RoomAID    string
	RoomBID    string
	AnchorAID  string
	AnchorBID  string
	Status     PKStatus
	ScoreA     int64
	ScoreB     int64
	StartedAt  *time.Time
	EndsAt     *time.Time
	Winner     PKWinner
	CreatedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time
}

// NewPKSession creates a pending PK session.
func NewPKSession(id, roomA, roomB, anchorA, anchorB string) *PKSession {
	now := time.Now()
	return &PKSession{
		ID:        id,
		RoomAID:   roomA,
		RoomBID:   roomB,
		AnchorAID: anchorA,
		AnchorBID: anchorB,
		Status:    PKStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Accept moves a pending session to matched.
func (p *PKSession) Accept() error {
	if p.Status != PKStatusPending {
		return ErrInvalidTransition
	}
	p.Status = PKStatusMatched
	p.UpdatedAt = time.Now()
	return nil
}

// Start begins the battle and arms the countdown. It transitions matched -> ongoing.
func (p *PKSession) Start(duration time.Duration) error {
	if p.Status != PKStatusMatched {
		return ErrInvalidTransition
	}
	now := time.Now()
	end := now.Add(duration)
	p.Status = PKStatusOngoing
	p.StartedAt = &now
	p.EndsAt = &end
	p.UpdatedAt = now
	return nil
}

// AddScore increments one side's score. Only valid while ongoing.
func (p *PKSession) AddScore(side PKSide, amount int64) error {
	if p.Status != PKStatusOngoing {
		return ErrInvalidTransition
	}
	switch side {
	case PKSideA:
		p.ScoreA += amount
	case PKSideB:
		p.ScoreB += amount
	default:
		return ErrInvalidSide
	}
	p.UpdatedAt = time.Now()
	return nil
}

// Finish ends the battle and decides the winner. Transitions ongoing -> finished.
func (p *PKSession) Finish() error {
	if p.Status != PKStatusOngoing {
		return ErrInvalidTransition
	}
	now := time.Now()
	p.Status = PKStatusFinished
	p.FinishedAt = &now
	p.UpdatedAt = now
	switch {
	case p.ScoreA > p.ScoreB:
		p.Winner = PKWinnerA
	case p.ScoreB > p.ScoreA:
		p.Winner = PKWinnerB
	default:
		p.Winner = PKWinnerDraw
	}
	return nil
}

// SideOfRoom returns which side a room belongs to, and whether it participates.
func (p *PKSession) SideOfRoom(roomID string) (PKSide, bool) {
	if roomID == p.RoomAID {
		return PKSideA, true
	}
	if roomID == p.RoomBID {
		return PKSideB, true
	}
	return "", false
}

// IsOngoing reports whether scoring is currently open.
func (p *PKSession) IsOngoing() bool {
	return p.Status == PKStatusOngoing
}
