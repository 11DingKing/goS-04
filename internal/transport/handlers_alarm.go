package transport

import (
	"net/http"

	"patrol-platform/internal/service"

	"patrol-platform/internal/domain"
)

// --- Alarm ---

func (s *Server) ReportAlarm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID      string   `json:"task_id"`
		ReporterID  string   `json:"reporter_id"`
		Type        string   `json:"type"`
		Description string   `json:"description"`
		MediaRefs   []string `json:"media_refs"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	alarm, err := s.alarmSvc.Report(r.Context(), req.ReporterID, req.TaskID,
		domain.AlarmType(req.Type), req.Description, req.MediaRefs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeCreated(w, alarm)
}

func (s *Server) ListAlarms(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.alarmSvc.ListAlarms(r.Context()))
}

func (s *Server) GetAlarm(w http.ResponseWriter, r *http.Request) {
	alarm, err := s.alarmSvc.GetAlarm(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, alarm)
}

func (s *Server) ReviewAlarm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReviewerID string `json:"reviewer_id"`
		Level      int    `json:"level"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	alarm, err := s.alarmSvc.Review(r.Context(), r.PathValue("id"), req.ReviewerID, domain.AlarmLevel(req.Level))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, alarm)
}

func (s *Server) AssignAlarm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReviewerID string `json:"reviewer_id"`
		HandlerID  string `json:"handler_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	alarm, err := s.alarmSvc.Assign(r.Context(), r.PathValue("id"), req.ReviewerID, req.HandlerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, alarm)
}

func (s *Server) AcceptAlarm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HandlerID string `json:"handler_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	alarm, err := s.alarmSvc.Accept(r.Context(), r.PathValue("id"), req.HandlerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, alarm)
}

func (s *Server) StartAlarm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HandlerID string `json:"handler_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	alarm, err := s.alarmSvc.StartHandling(r.Context(), r.PathValue("id"), req.HandlerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, alarm)
}

func (s *Server) ResolveAlarm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HandlerID  string `json:"handler_id"`
		Resolution string `json:"resolution"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	alarm, err := s.alarmSvc.Resolve(r.Context(), r.PathValue("id"), req.HandlerID, req.Resolution)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, alarm)
}

// --- Terminal ---

func (s *Server) RegisterTerminal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string `json:"id"`
		PatrollerID string `json:"patroller_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	term, err := s.terminalSvc.Register(r.Context(), req.ID, req.PatrollerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeCreated(w, term)
}

func (s *Server) GetTerminal(w http.ResponseWriter, r *http.Request) {
	term, err := s.terminalSvc.GetTerminal(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, term)
}

func (s *Server) SetOffline(w http.ResponseWriter, r *http.Request) {
	term, err := s.terminalSvc.SetOffline(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, term)
}

func (s *Server) SetOnline(w http.ResponseWriter, r *http.Request) {
	term, err := s.terminalSvc.SetOnline(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, term)
}

func (s *Server) SyncOffline(w http.ResponseWriter, r *http.Request) {
	result, err := s.terminalSvc.SyncOffline(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, result)
}

func (s *Server) CacheClockIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID      string  `json:"task_id"`
		KeyPointID  string  `json:"key_point_id"`
		PatrollerID string  `json:"patroller_id"`
		Lat         float64 `json:"lat"`
		Lng         float64 `json:"lng"`
		PhotoRef    string  `json:"photo_ref"`
		RequestID   string  `json:"request_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	err := s.terminalSvc.CacheClockIn(r.Context(), r.PathValue("id"), service.ClockInRequest{
		TaskID:      req.TaskID,
		KeyPointID:  req.KeyPointID,
		PatrollerID: req.PatrollerID,
		Lat:         req.Lat,
		Lng:         req.Lng,
		PhotoRef:    req.PhotoRef,
		RequestID:   req.RequestID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, map[string]string{"status": "cached"})
}

func (s *Server) CacheTrack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID     string  `json:"task_id"`
		TerminalID string  `json:"terminal_id"`
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
		Sequence   int     `json:"sequence"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	err := s.terminalSvc.CacheTrack(r.Context(), r.PathValue("id"), service.TrackRequest{
		TaskID:     req.TaskID,
		TerminalID: req.TerminalID,
		Lat:        req.Lat,
		Lng:        req.Lng,
		Sequence:   req.Sequence,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, map[string]string{"status": "cached"})
}
