package domain

import (
	"fmt"
	"time"
)

// AlarmType categorises the nature of a reported incident.
type AlarmType string

const (
	AlarmTypeFire     AlarmType = "fire"     // 火情
	AlarmTypePoaching AlarmType = "poaching" // 盗猎
	AlarmTypeGrazing  AlarmType = "grazing"  // 越界放牧
)

// AlarmLevel indicates severity; level 1 requires on-site response within two hours.
type AlarmLevel int

const (
	LevelOne   AlarmLevel = 1
	LevelTwo   AlarmLevel = 2
	LevelThree AlarmLevel = 3
)

// AlarmStatus represents the lifecycle state of an alarm event.
type AlarmStatus string

const (
	AlarmReported   AlarmStatus = "reported"    // 已上报
	AlarmReviewed   AlarmStatus = "reviewed"    // 已审核定级
	AlarmAssigned   AlarmStatus = "assigned"    // 已指派处置人
	AlarmAccepted   AlarmStatus = "accepted"    // 已接单
	AlarmInProgress AlarmStatus = "in_progress" // 处置中
	AlarmResolved   AlarmStatus = "resolved"    // 已处置
	AlarmEscalated  AlarmStatus = "escalated"   // 已升级
)

var alarmTransitions = map[AlarmStatus][]AlarmStatus{
	AlarmReported:   {AlarmReviewed, AlarmEscalated},
	AlarmReviewed:   {AlarmAssigned, AlarmEscalated},
	AlarmAssigned:   {AlarmAccepted, AlarmReviewed, AlarmEscalated},
	AlarmAccepted:   {AlarmInProgress, AlarmResolved, AlarmEscalated},
	AlarmInProgress: {AlarmResolved, AlarmEscalated},
	AlarmResolved:   {},
	AlarmEscalated:  {AlarmReviewed, AlarmAssigned},
}

// CanTransitionTo reports whether a status transition is valid.
func (s AlarmStatus) CanTransitionTo(target AlarmStatus) bool {
	for _, t := range alarmTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// Alarm is the aggregate root for a reported incident.
type Alarm struct {
	ID          string       `json:"id"`
	TaskID      string       `json:"task_id"`
	ReporterID  string       `json:"reporter_id"`
	Type        AlarmType    `json:"type"`
	Level       AlarmLevel   `json:"level"`
	Description string       `json:"description"`
	MediaRefs   []string     `json:"media_refs"`
	Status      AlarmStatus  `json:"status"`
	ReviewerID  string       `json:"reviewer_id"`
	HandlerID   string       `json:"handler_id"`
	AcceptedAt  time.Time    `json:"accepted_at"`
	ResolvedAt  time.Time    `json:"resolved_at"`
	CreatedAt   time.Time    `json:"created_at"`
	ReviewedAt  time.Time    `json:"reviewed_at"`
	EscalatedAt time.Time    `json:"escalated_at"`
	Escalated   bool         `json:"escalated"`
	Version     int          `json:"version"`
	Audit       []AuditEntry `json:"audit"`
}

// AddAudit appends an audit entry.
func (a *Alarm) AddAudit(action, actor, detail string, ts time.Time) {
	a.Audit = append(a.Audit, AuditEntry{
		Action: action, ActorID: actor, Detail: detail, Timestamp: ts,
	})
}

// Review transitions the alarm to reviewed with a level classification.
func (a *Alarm) Review(reviewerID string, level AlarmLevel, now time.Time) {
	a.Status = AlarmReviewed
	a.Level = level
	a.ReviewerID = reviewerID
	a.ReviewedAt = now
	a.Version++
	a.AddAudit("review", reviewerID, fmt.Sprintf("classified as level %d", level), now)
}

// Assign transitions the alarm to assigned with a recommended handler.
func (a *Alarm) Assign(reviewerID, handlerID string, now time.Time) {
	a.Status = AlarmAssigned
	a.HandlerID = handlerID
	a.Version++
	a.AddAudit("assign", reviewerID, fmt.Sprintf("assigned to handler %s", handlerID), now)
}

// CanAccept validates whether a handler may accept this alarm.
func (a *Alarm) CanAccept(handlerID string) error {
	if a.Status != AlarmAssigned && a.Status != AlarmReviewed {
		return fmt.Errorf("%w: alarm is %s", ErrAlreadyAccepted, a.Status)
	}
	return nil
}

// Accept transitions the alarm to accepted.
func (a *Alarm) Accept(handlerID string, now time.Time) {
	a.Status = AlarmAccepted
	a.HandlerID = handlerID
	a.AcceptedAt = now
	a.Version++
	a.AddAudit("accept", handlerID, "alarm accepted", now)
}

// StartHandling transitions the alarm to in_progress.
func (a *Alarm) StartHandling(handlerID string, now time.Time) {
	a.Status = AlarmInProgress
	a.Version++
	a.AddAudit("start", handlerID, "handling started", now)
}

// Resolve transitions the alarm to resolved.
func (a *Alarm) Resolve(handlerID, resolution string, now time.Time) {
	a.Status = AlarmResolved
	a.ResolvedAt = now
	a.Version++
	a.AddAudit("resolve", handlerID, resolution, now)
}

// Escalate marks the alarm as escalated and notifies the chief.
func (a *Alarm) Escalate(reason string, now time.Time) {
	a.Status = AlarmEscalated
	a.Escalated = true
	a.EscalatedAt = now
	a.Version++
	a.AddAudit("escalate", "system", reason, now)
}

// IsOverdue checks whether a level-1 alarm has exceeded its response window.
func (a *Alarm) IsOverdue(now time.Time, timeout time.Duration) bool {
	if a.Level != LevelOne {
		return false
	}
	if a.Status == AlarmResolved || a.Status == AlarmEscalated {
		return false
	}
	base := a.ReviewedAt
	if base.IsZero() {
		base = a.CreatedAt
	}
	return now.Sub(base) > timeout
}

// RollbackToReview resets an assigned alarm back to the reviewed state
// when the handler does not accept within the timeout window.
func (a *Alarm) RollbackToReview(reason string, now time.Time) {
	a.Status = AlarmReviewed
	a.HandlerID = ""
	a.Version++
	a.AddAudit("rollback", "system", reason, now)
}

// Clone returns a deep copy of the alarm.
func (a *Alarm) Clone() *Alarm {
	cp := *a
	if a.MediaRefs != nil {
		cp.MediaRefs = append([]string(nil), a.MediaRefs...)
	}
	if a.Audit != nil {
		cp.Audit = append([]AuditEntry(nil), a.Audit...)
	}
	return &cp
}
