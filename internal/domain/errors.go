package domain

import "errors"

var (
	ErrNotFound           = errors.New("entity not found")
	ErrInvalidTransition  = errors.New("invalid state transition")
	ErrAlreadyClaimed     = errors.New("task already claimed by another patroller")
	ErrAlreadyAccepted    = errors.New("alarm already accepted by another handler")
	ErrClockInGapExceeded = errors.New("clock-in interval exceeds six hours")
	ErrDualPatrolRequired = errors.New("core zone requires dual patrol and advance reporting")
	ErrNotAuthorized      = errors.New("actor not authorized for this operation")
	ErrDuplicateRequest   = errors.New("duplicate request already processed")
	ErrAlarmOverdue       = errors.New("alarm processing is overdue")
	ErrTerminalOffline    = errors.New("terminal is offline, data cached locally")
)
