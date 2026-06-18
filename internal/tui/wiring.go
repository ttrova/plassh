package tui

import (
	"time"

	"plassh/internal/presence"
)

// PresenceEvent is the exported alias of the internal presenceEvent so the ssh
// package can construct the channel the model consumes.
type PresenceEvent = presenceEvent

// NewPresenceEvent wraps a presence.Update for delivery to the model.
func NewPresenceEvent(u presence.Update) PresenceEvent {
	return presenceEvent{Update: u}
}

// WithExisting seeds the model's remote map from presence already in Redis,
// skipping this session's own id and any "gone" markers.
func WithExisting(m Model, existing []presence.Update, selfID string) Model {
	for _, u := range existing {
		if u.ID == selfID || u.Gone {
			continue
		}
		m.remotes[u.ID] = remote{x: u.X, y: u.Y, color: u.Color, name: u.Name, lastSeen: time.Now()}
	}
	return m
}
