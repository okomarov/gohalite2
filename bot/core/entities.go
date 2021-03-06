package core

import (
	"fmt"
)

// ------------------------------------------------------

type Entity interface {
	Type()							EntityType
	GetId()							int
	GetX()							float64
	GetY()							float64
	GetRadius()						float64
	Angle(other Entity)				int
	Dist(other Entity)				float64
	ApproachDist(other Entity)		float64							// ApproachDist(): distance from my CENTRE to target's EDGE.
	Alive()							bool
	String()						string
}

func EntitiesDist(a, b Entity) float64 {
	if a.Type() == NOTHING || b.Type() == NOTHING {
		return 0
	}
	return Dist(a.GetX(), a.GetY(), b.GetX(), b.GetY())
}

func EntitiesApproachDist(a, b Entity) float64 {					// Centre to edge dist. Don't change now...
	return EntitiesDist(a, b) - b.GetRadius()
}

func EntitiesCollide(a, b Entity) bool {
	if a.Type() == NOTHING || b.Type() == NOTHING {
		return false
	}
	return EntitiesDist(a, b) <= a.GetRadius() + b.GetRadius()
}

func EntitiesAngle(a, b Entity) int {
	if a.Type() == NOTHING || b.Type() == NOTHING {
		panic("EntitiesAngle() called with NOTHING entity")
	}
	return Angle(a.GetX(), a.GetY(), b.GetX(), b.GetY())
}

// ------------------------------------------------------

type Planet struct {
	Id								int
	X								float64
	Y								float64
	HP								int
	Radius							float64
	DockingSpots					int
	CurrentProduction				int
	Owned							bool
	Owner							int			// Protocol will send 0 if not owned at all, but we "correct" this to -1
	DockedShips						int			// The ships themselves can be accessed via game.dockMap[]
}

func (p *Planet) OpenSpots() int {
	return p.DockingSpots - p.DockedShips
}

func (p *Planet) IsFull() bool {
	return p.DockedShips >= p.DockingSpots
}

// ------------------------------------------------------

type Ship struct {
	Id					int
	Owner				int
	X					float64
	Y					float64
	HP					int
	DockedStatus		DockedStatus
	DockedPlanet		int
	DockingProgress		int

	Birth				int			// Turn this ship was first seen
	SpawnX				float64
	SpawnY				float64

	Firing				bool		// Whether the ship will fire at Time 0 this turn (unless it docks)
	Doomed				bool		// Whether the ship will die at Time 0 this turn (unless nearby enemy ships dock)

	Dx					float64
	Dy					float64
	LastSpeed			int
	LastAngle			int

	ClosestEnemy		*Ship
}

func (s *Ship) CanDock(p *Planet) bool {
	if s.Alive() && p.Alive() && p.IsFull() == false && (p.Owned == false || p.Owner == s.Owner) {
		return s.ApproachDist(p) < DOCKING_RADIUS + SHIP_RADIUS
	}
	return false
}

func (s *Ship) CanMove() bool {
	return s.DockedStatus == UNDOCKED
}

func (s *Ship) ShotsToKill() int {
	return ((s.HP + 63) / 64)		// Exploiting integer division
}

func (s *Ship) VagueDirection() Edge {

	// Was the ship's last move mostly up, down, left, or right?
	// Note that stationary ships return RIGHT.

	if s.LastSpeed == 0 { return RIGHT }
	if s.LastAngle > 360 { panic("VagueDirection(): ship had angle > 360") }

	if s.LastAngle >= 315 || s.LastAngle < 45 { return RIGHT }
	if s.LastAngle >= 135 && s.LastAngle < 225 { return LEFT }
	if s.LastAngle < 180 { return BOTTOM }
	return TOP
}

func (s *Ship) find_closest_enemy(game *Game) *Ship {

	var ret *Ship

	closest_dist := 9999.9

	for _, other := range game.AllShips() {

		if other.Owner == s.Owner {
			continue
		}

		d := s.Dist(other)
		if d < closest_dist {
			ret = other
			closest_dist = d
		}
	}

	return ret
}

func (s *Ship) fudge_dock_status() {

	// Since docking status is the first thing updated in a turn,
	// we should really push it forward one step. We might get
	// a command in an iteration earlier due to this.
	//
	// The parser calls this.

	switch s.DockedStatus {

	case DOCKING:

		s.DockingProgress--
		if s.DockingProgress == 0 {
			s.DockedStatus = DOCKED
		}

	case UNDOCKING:

		s.DockingProgress--
		if s.DockingProgress == 0 {
			s.DockedStatus = UNDOCKED
			s.DockedPlanet = -1						// The parser will fix up the planet's info.
		}
	}
}

// ------------------------------------------------------

type Point struct {
	X								float64
	Y								float64
}

type Port struct {		// A port is like a point but flagged as being somewhere we want to dock at.
	X								float64
	Y								float64
	PlanetID						int
}

type Circle struct {	// Might be useful for putting into the avoid_list in some cases.
	X								float64
	Y								float64
	Radius							float64
}

type NothingType struct {}

var Nothing *NothingType = &NothingType{}

// ------------------------------------------------------

// Interface satisfiers....

func (e *Ship) Type() EntityType { return SHIP }
func (e *Point) Type() EntityType { return POINT }
func (e *Port) Type() EntityType { return PORT }
func (e *Planet) Type() EntityType { return PLANET }
func (e *Circle) Type() EntityType { return CIRCLE }
func (e *NothingType) Type() EntityType { return NOTHING }

func (e *Ship) GetId() int { return e.Id }
func (e *Point) GetId() int { return -1 }
func (e *Port) GetId() int { return e.PlanetID }
func (e *Planet) GetId() int { return e.Id }
func (e *Circle) GetId() int { return -1 }
func (e *NothingType) GetId() int { return -1 }

func (e *Ship) GetX() float64 { return e.X }
func (e *Point) GetX() float64 { return e.X }
func (e *Port) GetX() float64 { return e.X }
func (e *Planet) GetX() float64 { return e.X }
func (e *Circle) GetX() float64 { return e.X }
func (e *NothingType) GetX() float64 { panic("GetX() called on NOTHING entity") }

func (e *Ship) GetY() float64 { return e.Y }
func (e *Point) GetY() float64 { return e.Y }
func (e *Port) GetY() float64 { return e.Y }
func (e *Planet) GetY() float64 { return e.Y }
func (e *Circle) GetY() float64 { return e.Y }
func (e *NothingType) GetY() float64 { panic("GetY() called on NOTHING entity") }

func (e *Ship) GetRadius() float64 { return SHIP_RADIUS }
func (e *Point) GetRadius() float64 { return 0 }
func (e *Port) GetRadius() float64 { return 0 }
func (e *Planet) GetRadius() float64 { return e.Radius }
func (e *Circle) GetRadius() float64 { return e.Radius }
func (e *NothingType) GetRadius() float64 { return 0 }

func (e *Ship) Angle(other Entity) int { return EntitiesAngle(e, other) }
func (e *Point) Angle(other Entity) int { return EntitiesAngle(e, other) }
func (e *Port) Angle(other Entity) int { return EntitiesAngle(e, other) }
func (e *Planet) Angle(other Entity) int { return EntitiesAngle(e, other) }
func (e *Circle) Angle(other Entity) int { return EntitiesAngle(e, other) }
func (e *NothingType) Angle(other Entity) int { return EntitiesAngle(e, other) }						// Will panic

func (e *Ship) Dist(other Entity) float64 { return EntitiesDist(e, other) }
func (e *Point) Dist(other Entity) float64 { return EntitiesDist(e, other) }
func (e *Port) Dist(other Entity) float64 { return EntitiesDist(e, other) }
func (e *Planet) Dist(other Entity) float64 { return EntitiesDist(e, other) }
func (e *Circle) Dist(other Entity) float64 { return EntitiesDist(e, other) }
func (e *NothingType) Dist(other Entity) float64 { return EntitiesDist(e, other) }

func (e *Ship) ApproachDist(other Entity) float64 { return EntitiesApproachDist(e, other) }
func (e *Point) ApproachDist(other Entity) float64 { return EntitiesApproachDist(e, other) }
func (e *Port) ApproachDist(other Entity) float64 { return EntitiesApproachDist(e, other) }
func (e *Planet) ApproachDist(other Entity) float64 { return EntitiesApproachDist(e, other) }
func (e *Circle) ApproachDist(other Entity) float64 { return EntitiesApproachDist(e, other) }
func (e *NothingType) ApproachDist(other Entity) float64 { return EntitiesApproachDist(e, other) }

func (e *Ship) Alive() bool { return e.HP > 0 }
func (e *Point) Alive() bool { return true }
func (e *Port) Alive() bool { return true }
func (e *Planet) Alive() bool { return e.HP > 0 }
func (e *Circle) Alive() bool { return true }
func (e *NothingType) Alive() bool { return false }

func (e *Ship) String() string { return fmt.Sprintf("Ship %d [%d,%d]", e.Id, int(e.X), int(e.Y)) }
func (e *Point) String() string { return fmt.Sprintf("Point [%d,%d]", int(e.X), int(e.Y)) }
func (e *Port) String() string { return fmt.Sprintf("Port [%d,%d]", int(e.X), int(e.Y)) }
func (e *Planet) String() string { return fmt.Sprintf("Planet %d [%d,%d]", e.Id, int(e.X), int(e.Y)) }
func (e *Circle) String() string { return fmt.Sprintf("Circle [%d,%d;%d]", int(e.X), int(e.Y), int(e.Radius)) }
func (e *NothingType) String() string { return "null entity" }
