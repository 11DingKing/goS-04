package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"patrol-platform/internal/domain"
	"patrol-platform/internal/service"
)

// Server holds service references and provides HTTP handler methods.
type Server struct {
	gridSvc     *service.GridService
	taskSvc     *service.TaskService
	alarmSvc    *service.AlarmService
	terminalSvc *service.TerminalService
}

func NewServer(gridSvc *service.GridService, taskSvc *service.TaskService, alarmSvc *service.AlarmService, terminalSvc *service.TerminalService) *Server {
	return &Server{gridSvc: gridSvc, taskSvc: taskSvc, alarmSvc: alarmSvc, terminalSvc: terminalSvc}
}

type apiResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data,omitempty"`
	Err  string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: data})
}

func writeCreated(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, apiResponse{OK: true, Data: data})
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, mapErrorToStatus(err), apiResponse{Err: err.Error()})
}

func mapErrorToStatus(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrAlreadyClaimed),
		errors.Is(err, domain.ErrAlreadyAccepted),
		errors.Is(err, domain.ErrDuplicateRequest):
		return http.StatusConflict
	case errors.Is(err, domain.ErrNotAuthorized):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrClockInGapExceeded),
		errors.Is(err, domain.ErrDualPatrolRequired),
		errors.Is(err, domain.ErrAlarmOverdue),
		errors.Is(err, domain.ErrTerminalOffline):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func decodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// --- Health ---

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"status": "ok"})
}

// --- Grid ---

func (s *Server) CreateGrid(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Area         string   `json:"area"`
		Zone         string   `json:"zone"`
		PatrollerIDs []string `json:"patroller_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	grid, err := s.gridSvc.CreateGrid(r.Context(), &domain.Grid{
		ID:           req.ID,
		Name:         req.Name,
		Area:         req.Area,
		Zone:         domain.Zone(req.Zone),
		PatrollerIDs: req.PatrollerIDs,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeCreated(w, grid)
}

func (s *Server) ListGrids(w http.ResponseWriter, r *http.Request) {
	grids, err := s.gridSvc.ListGrids(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, grids)
}

func (s *Server) GetGrid(w http.ResponseWriter, r *http.Request) {
	grid, err := s.gridSvc.GetGrid(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, grid)
}

// --- Task ---

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DispatcherID string            `json:"dispatcher_id"`
		GridID       string            `json:"grid_id"`
		Title        string            `json:"title"`
		Priority     string            `json:"priority"`
		KeyPoints    []domain.KeyPoint `json:"key_points"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.Dispatch(r.Context(), req.DispatcherID, req.GridID, req.Title,
		domain.Priority(req.Priority), req.KeyPoints)
	if err != nil {
		writeError(w, err)
		return
	}
	writeCreated(w, task)
}

func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.taskSvc.ListTasks(r.Context()))
}

func (s *Server) GetTask(w http.ResponseWriter, r *http.Request) {
	task, err := s.taskSvc.GetTask(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}

func (s *Server) ClaimTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatrollerID string `json:"patroller_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.Claim(r.Context(), r.PathValue("id"), req.PatrollerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}

func (s *Server) PreReportTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReporterID string `json:"reporter_id"`
		PartnerID  string `json:"partner_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.PreReport(r.Context(), r.PathValue("id"), req.ReporterID, req.PartnerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}

func (s *Server) StartTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatrollerID string `json:"patroller_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.Start(r.Context(), r.PathValue("id"), req.PatrollerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}

func (s *Server) ClockIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
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
	ci, err := s.taskSvc.RecordClockIn(r.Context(), service.ClockInRequest{
		TaskID:      r.PathValue("id"),
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
	writeOK(w, ci)
}

func (s *Server) RecordTrack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TerminalID string  `json:"terminal_id"`
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
		Sequence   int     `json:"sequence"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	err := s.taskSvc.RecordTrack(r.Context(), service.TrackRequest{
		TaskID:     r.PathValue("id"),
		TerminalID: req.TerminalID,
		Lat:        req.Lat,
		Lng:        req.Lng,
		Sequence:   req.Sequence,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, map[string]string{"status": "recorded"})
}

func (s *Server) CompleteTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatrollerID string `json:"patroller_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.Complete(r.Context(), r.PathValue("id"), req.PatrollerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}

func (s *Server) RollbackTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor  string `json:"actor"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.RollbackTask(r.Context(), r.PathValue("id"), req.Actor, req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}

func (s *Server) CancelTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor string `json:"actor"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.taskSvc.Cancel(r.Context(), r.PathValue("id"), req.Actor)
	if err != nil {
		writeError(w, err)
		return
	}
	writeOK(w, task)
}
