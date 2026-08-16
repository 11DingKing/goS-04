package domain

// Zone represents the protection classification of a grid area.
type Zone string

const (
	ZoneCore       Zone = "core"       // 核心区
	ZoneBuffer     Zone = "buffer"     // 缓冲区
	ZoneExperiment Zone = "experiment" // 实验区
)

// Grid represents a patrol grid within a management area.
type Grid struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Area         string   `json:"area"`
	Zone         Zone     `json:"zone"`
	PatrollerIDs []string `json:"patroller_ids"`
}

// HasPatroller reports whether a patroller is assigned to this grid.
func (g *Grid) HasPatroller(id string) bool {
	for _, p := range g.PatrollerIDs {
		if p == id {
			return true
		}
	}
	return false
}

// Clone returns a deep copy of the grid.
func (g *Grid) Clone() *Grid {
	cp := *g
	if g.PatrollerIDs != nil {
		cp.PatrollerIDs = append([]string(nil), g.PatrollerIDs...)
	}
	return &cp
}
