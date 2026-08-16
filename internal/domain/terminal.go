package domain

import "time"

// Terminal represents a handheld device carried by a patroller.
type Terminal struct {
	ID              string       `json:"id"`
	PatrollerID     string       `json:"patroller_id"`
	Online          bool         `json:"online"`
	LastSeen        time.Time    `json:"last_seen"`
	PendingClockIns []ClockIn    `json:"pending_clock_ins"`
	PendingTracks   []TrackPoint `json:"pending_tracks"`
}

// GoOffline marks the terminal as offline and records the transition time.
func (term *Terminal) GoOffline(now time.Time) {
	term.Online = false
	term.LastSeen = now
}

// GoOnline marks the terminal as back online.
func (term *Terminal) GoOnline(now time.Time) {
	term.Online = true
	term.LastSeen = now
}

// CacheClockIn stores a clock-in record for later synchronisation.
func (term *Terminal) CacheClockIn(ci ClockIn) {
	ci.Offline = true
	term.PendingClockIns = append(term.PendingClockIns, ci)
}

// CacheTrack stores a track point for later synchronisation.
func (term *Terminal) CacheTrack(tp TrackPoint) {
	term.PendingTracks = append(term.PendingTracks, tp)
}

// HasPendingData reports whether the terminal holds offline-cached records.
func (term *Terminal) HasPendingData() bool {
	return len(term.PendingClockIns) > 0 || len(term.PendingTracks) > 0
}

// ClearPending removes all cached records after successful synchronisation.
func (term *Terminal) ClearPending() {
	term.PendingClockIns = nil
	term.PendingTracks = nil
}

// Clone returns a deep copy of the terminal.
func (term *Terminal) Clone() *Terminal {
	cp := *term
	if term.PendingClockIns != nil {
		cp.PendingClockIns = append([]ClockIn(nil), term.PendingClockIns...)
	}
	if term.PendingTracks != nil {
		cp.PendingTracks = append([]TrackPoint(nil), term.PendingTracks...)
	}
	return &cp
}
