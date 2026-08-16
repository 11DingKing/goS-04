package transport

import (
	"net/http"

	"patrol-platform/internal/service"
)

// NewRouter builds the HTTP mux with all API routes registered.
func NewRouter(gridSvc *service.GridService, taskSvc *service.TaskService, alarmSvc *service.AlarmService, terminalSvc *service.TerminalService) http.Handler {
	srv := NewServer(gridSvc, taskSvc, alarmSvc, terminalSvc)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", srv.Health)

	mux.HandleFunc("POST /api/grids", srv.CreateGrid)
	mux.HandleFunc("GET /api/grids", srv.ListGrids)
	mux.HandleFunc("GET /api/grids/{id}", srv.GetGrid)

	mux.HandleFunc("POST /api/tasks", srv.CreateTask)
	mux.HandleFunc("GET /api/tasks", srv.ListTasks)
	mux.HandleFunc("GET /api/tasks/{id}", srv.GetTask)
	mux.HandleFunc("POST /api/tasks/{id}/claim", srv.ClaimTask)
	mux.HandleFunc("POST /api/tasks/{id}/pre-report", srv.PreReportTask)
	mux.HandleFunc("POST /api/tasks/{id}/start", srv.StartTask)
	mux.HandleFunc("POST /api/tasks/{id}/clockin", srv.ClockIn)
	mux.HandleFunc("POST /api/tasks/{id}/track", srv.RecordTrack)
	mux.HandleFunc("POST /api/tasks/{id}/complete", srv.CompleteTask)
	mux.HandleFunc("POST /api/tasks/{id}/rollback", srv.RollbackTask)
	mux.HandleFunc("POST /api/tasks/{id}/cancel", srv.CancelTask)

	mux.HandleFunc("POST /api/alarms", srv.ReportAlarm)
	mux.HandleFunc("GET /api/alarms", srv.ListAlarms)
	mux.HandleFunc("GET /api/alarms/{id}", srv.GetAlarm)
	mux.HandleFunc("POST /api/alarms/{id}/review", srv.ReviewAlarm)
	mux.HandleFunc("POST /api/alarms/{id}/assign", srv.AssignAlarm)
	mux.HandleFunc("POST /api/alarms/{id}/accept", srv.AcceptAlarm)
	mux.HandleFunc("POST /api/alarms/{id}/start", srv.StartAlarm)
	mux.HandleFunc("POST /api/alarms/{id}/resolve", srv.ResolveAlarm)

	mux.HandleFunc("POST /api/terminals", srv.RegisterTerminal)
	mux.HandleFunc("GET /api/terminals/{id}", srv.GetTerminal)
	mux.HandleFunc("POST /api/terminals/{id}/offline", srv.SetOffline)
	mux.HandleFunc("POST /api/terminals/{id}/online", srv.SetOnline)
	mux.HandleFunc("POST /api/terminals/{id}/sync", srv.SyncOffline)
	mux.HandleFunc("POST /api/terminals/{id}/clockin", srv.CacheClockIn)
	mux.HandleFunc("POST /api/terminals/{id}/track", srv.CacheTrack)

	return mux
}
