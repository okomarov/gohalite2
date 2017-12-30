package pilot

import (
	"sort"

	hal "../core"
)

func (self *Pilot) SetTurnTarget() {				// Set our short term tactical target.

	if self.DockedStatus != hal.UNDOCKED || self.Target.Type() != hal.PLANET {
		return
	}

	planet := self.Target.(hal.Planet)

	// Is the planet far away?

	if self.ApproachDist(planet) > 100 {
		return
	}

	// Are there enemy ships near the planet? Includes docked enemies.

	enemies := self.Game.EnemiesNearPlanet(planet)

	if len(enemies) > 0 {

		// Find closest...

		sort.Slice(enemies, func(a, b int) bool {
			return enemies[a].Dist(self.Ship) < enemies[b].Dist(self.Ship)
		})

		self.Target = enemies[0]
		return
	}

	// Otherwise, just return (leaving the Target as the planet).

	return
}

func (self *Pilot) PlanChase(avoid_list []hal.Entity) {

	// We have our target, but what are we doing about it?

	if self.DockedStatus != hal.UNDOCKED {
		return
	}

	switch self.Target.Type() {

	case hal.PLANET:

		self.PlanetApproachForDock(avoid_list)

	case hal.SHIP:

		other_ship := self.Target.(hal.Ship)
		self.EngageShip(other_ship, avoid_list)

	case hal.POINT:

		point := self.Target.(hal.Point)

		side := self.DecideSideFromTarget()
		speed, degrees, err := self.GetCourse(point, avoid_list, side)

		if err != nil {
			self.Target = hal.Nothing{}
		} else {
			self.PlanThrust(speed, degrees)
		}

	case hal.PORT:

		port := self.Target.(hal.Port)

		planet, ok := self.Game.GetPlanet(port.PlanetID)

		if ok == false {
			self.Target = hal.Nothing{}
			return
		}

		if self.CanDock(planet) {
			self.PlanDock(planet)
			return
		}

		side := self.DecideSideFromTarget()
		speed, degrees, err := self.GetCourse(port, avoid_list, side)

		if err != nil {
			self.Target = hal.Nothing{}
		} else {
			self.PlanThrust(speed, degrees)
		}
	}
}

func (self *Pilot) EngageShip(enemy_ship hal.Ship, avoid_list []hal.Entity) {

	// Flee if we're already in weapons range...

	if self.Dist(enemy_ship) < hal.WEAPON_RANGE + hal.SHIP_RADIUS * 2 {
		self.EngageShipFlee(enemy_ship, avoid_list)
		return
	}

	// Approach...

	self.EngageShipApproach(enemy_ship, avoid_list)
}

func (self *Pilot) EngageShipApproach(enemy_ship hal.Ship, avoid_list []hal.Entity) {
	side := self.DecideSideFromTarget()
	speed, degrees, err := self.GetApproach(enemy_ship, self.EnemyApproachDist, avoid_list, side)
	if err != nil { self.Message = MSG_RECURSION }
	self.PlanThrust(speed, degrees)
}

func (self *Pilot) EngageShipFlee(enemy_ship hal.Ship, avoid_list []hal.Entity) {

	// We were already within range of our target ship, so we will definitely attack it this turn.
	// We can therefore back off.

	angle := self.Angle(enemy_ship) + 180

	x2, y2 := hal.Projection(self.X, self.Y, 7, angle)
	flee_point := hal.Point{x2, y2}

	side := self.DecideSideFromTarget()
	speed, degrees, err := self.GetApproach(flee_point, 1, avoid_list, side)
	if err != nil { self.Message = MSG_RECURSION }
	self.PlanThrust(speed, degrees)
}

func (self *Pilot) PlanetApproachForDock(avoid_list []hal.Entity) {

	if self.Target.Type() != hal.PLANET {
		self.Log("PlanetApproachForDock() called but target wasn't a planet.")
		return
	}

	planet := self.Target.(hal.Planet)

	if self.CanDock(planet) {
		self.PlanDock(planet)
		return
	}

	side := self.DecideSideFromTarget()
	speed, degrees, err := self.GetApproach(planet, hal.DOCKING_RADIUS + hal.SHIP_RADIUS - 0.001, avoid_list, side)
	if err != nil { self.Message = MSG_RECURSION }
	self.PlanThrust(speed, degrees)
}
