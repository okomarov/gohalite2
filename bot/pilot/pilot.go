package pilot

import (
	"fmt"
	"sort"

	atc "../atc"
	hal "../core"
)

// The point of making Pilot its own module is that the logic of dealing with targets is
// mostly independent of grand strategy. Still, there are a few things the Overmind may
// need back from us, hence the Overmind interface below which allows us to update it.

type Overmind interface {
	NotifyTargetChange(pilot *Pilot, old_target, new_target hal.Entity)
	NotifyDock(planet hal.Planet)
}

var ENEMY_SHIP_APPROACH_DIST float64 = 5.45			// GetApproach uses centre-to-edge distances, so 5.5ish

func SetEnemyShipApproachDist(d float64) {
	ENEMY_SHIP_APPROACH_DIST = d
}

type Pilot struct {
	hal.Ship
	Plan			string							// Our planned order, valid for 1 turn only
	HasExecuted		bool							// Have we actually "sent" the order? (Placed it in the game.orders map.)
	Overmind		Overmind
	Game			*hal.Game
	Target			hal.Entity						// Use a hal.Nothing{} struct for no target.
	NavStack		[]string
}

func NewPilot(sid int, game *hal.Game, overmind Overmind) *Pilot {
	ret := new(Pilot)
	ret.Overmind = overmind
	ret.Game = game
	ret.Id = sid
	ret.Target = hal.Nothing{}
	return ret
}

func (self *Pilot) Log(format_string string, args ...interface{}) {
	format_string = fmt.Sprintf("%v: ", self) + format_string
	self.Game.Log(format_string, args...)
}

func (self *Pilot) AddToNavStack(format_string string, args ...interface{}) {
	s := fmt.Sprintf(format_string, args...)
	self.NavStack = append(self.NavStack, s)
}

func (self *Pilot) LogNavStack() {
	self.Game.Log("%v Nav Stack:", self)
	for _, s := range self.NavStack {
		self.Game.Log("        %v", s)
	}
}

func (self *Pilot) ResetAndUpdate() bool {			// Doesn't clear Target. Return true if we still exist.

	self.NavStack = nil

	current_ship, alive := self.Game.GetShip(self.Id)

	if alive == false {
		self.SetTarget(hal.Nothing{})				// Means the overmind will be notified about our lack of target.
		return false
	}

	self.Ship = current_ship						// Don't do this until after the (possible) self.SetTarget() above.
	self.Plan = ""
	self.HasExecuted = false
	self.Game.RawOrder(self.Id, "")

	// Update the info about our target.

	if self.DockedStatus != hal.UNDOCKED {
		self.SetTarget(hal.Nothing{})
	}

	switch self.Target.Type() {

	case hal.SHIP:

		if self.Target.Alive() == false {
			self.SetTarget(hal.Nothing{})
		} else {
			var ok bool
			self.Target, ok = self.Game.GetShip(self.Target.GetId())
			if ok == false {
				self.SetTarget(hal.Nothing{})
			}
		}

	case hal.PLANET:

		if self.Target.Alive() == false {
			self.SetTarget(hal.Nothing{})
		} else {
			var ok bool
			self.Target, ok = self.Game.GetPlanet(self.Target.GetId())
			if ok == false {
				self.SetTarget(hal.Nothing{})
			}
		}
	}

	return true
}

func (self *Pilot) HasStationaryPlan() bool {		// true iff we DO have a plan, which doesn't move us.
	if self.Plan == "" {
		return false
	}
	speed, _ := self.CourseFromPlan()
	return speed == 0
}

func (self *Pilot) CourseFromPlan() (int, int) {
	return hal.CourseFromString(self.Plan)
}

func (self *Pilot) HasTarget() bool {				// We don't use nil ever, so we can e.g. call hal.Type()
	return self.Target.Type() != hal.NOTHING
}

func (self *Pilot) SetTarget(e hal.Entity) {		// So we can update Overmind's info.
	old_target := self.Target
	self.Target = e
	self.Overmind.NotifyTargetChange(self, old_target, e)
}

func (self *Pilot) ClosestPlanet() hal.Planet {
	return self.Game.ClosestPlanet(self)
}

func (self *Pilot) PlanChase(avoid_list []hal.Entity) {

	if self.DockedStatus != hal.UNDOCKED {
		return
	}

	if self.Target.Type() == hal.NOTHING {
		self.PlanThrust(0, 0, MSG_NO_TARGET)
		return
	}

	switch self.Target.Type() {

	case hal.PLANET:

		planet := self.Target.(hal.Planet)

		if self.ApproachDist(planet) <= 100 {
			self.EngagePlanet(avoid_list)
			return
		}

		side := self.DecideSideFromTarget()
		speed, degrees, err := self.GetApproach(planet, 4.45, avoid_list, side)

		if err != nil {
			self.SetTarget(hal.Nothing{})
		} else {
			self.PlanThrust(speed, degrees, MessageInt(planet.Id))
		}

	case hal.SHIP:

		other_ship := self.Target.(hal.Ship)
		self.EngageShip(other_ship, avoid_list)

	case hal.POINT:

		point := self.Target.(hal.Point)

		side := self.DecideSideFromTarget()
		speed, degrees, err := self.GetCourse(point, avoid_list, side)

		if err != nil {
			self.SetTarget(hal.Nothing{})
		} else {
			self.PlanThrust(speed, degrees, MSG_POINT_TARGET)
		}
	}
}

func (self *Pilot) EngagePlanet(avoid_list []hal.Entity) {

	// We are "close" to our target planet. Do something about this.
	// As of v32 or so, "close" is actually quite far.

	if self.Target.Type() != hal.PLANET {
		self.Log("EngagePlanet() called but target wasn't a planet.")
		return
	}

	planet := self.Target.(hal.Planet)

	// Are there enemy ships near the planet? Includes docked enemies.

	enemies := self.Game.EnemiesNearPlanet(planet)

	if len(enemies) > 0 {

		// Find closest...

		sort.Slice(enemies, func(a, b int) bool {
			return enemies[a].Dist(self.Ship) < enemies[b].Dist(self.Ship)
		})

		self.EngageShip(enemies[0], avoid_list)
		return
	}

	// Is it available for us to dock?

	if planet.Owned == false || (planet.Owner == self.Game.Pid() && planet.IsFull() == false) {
		self.FinalPlanetApproachForDock(avoid_list)
		return
	}

	// This function shouldn't have been called at all.

	self.Log("EngagePlanet() called but there's nothing to do here.")
	return
}

func (self *Pilot) EngageShip(enemy_ship hal.Ship, avoid_list []hal.Entity) {

	var side Side
	var msg MessageInt

	if self.Target.Type() == hal.PLANET {				// We're fighting a ship because it's near our target planet...
		side = self.DecideSide(enemy_ship, self.Target)
		msg = MSG_ORBIT_FIGHT
	} else {											// We're directly targeting a ship...
		side = self.DecideSideFromTarget()
		msg = MSG_ASSASSINATE
	}

	speed, degrees, err := self.GetApproach(enemy_ship, ENEMY_SHIP_APPROACH_DIST, avoid_list, side)

	if err != nil {
		self.PlanThrust(speed, degrees, MSG_RECURSION)
		if self.Target.Type() == hal.SHIP {				// This is just to maintain consistency with old version...
			self.SetTarget(hal.Nothing{})
		}
	} else {
		self.PlanThrust(speed, degrees, msg)
	}
}

func (self *Pilot) FinalPlanetApproachForDock(avoid_list []hal.Entity) {

	if self.Target.Type() != hal.PLANET {
		self.Log("FinalPlanetApproachForDock() called but target wasn't a planet.")
		return
	}

	planet := self.Target.(hal.Planet)

	if self.CanDock(planet) {
		self.PlanDock(planet)
		return
	}

	side := self.ArbitrarySide()											// FIXME?

	speed, degrees, err := self.GetApproach(planet, hal.DOCKING_RADIUS + hal.SHIP_RADIUS - 0.001, avoid_list, side)

	if err != nil {
		self.PlanThrust(speed, degrees, MSG_RECURSION)
	} else {
		self.PlanThrust(speed, degrees, MSG_DOCK_APPROACH)
	}
}

// -------------------------------------------------------------------

func (self *Pilot) PlanThrust(speed, degrees int, message MessageInt) {		// Send -1 as message for no message.

	for degrees < 0 {
		degrees += 360
	}

	degrees %= 360

	// We put some extra info into the angle, which we can see in the Chlorine replayer...

	if message >= 0 && message <= 180 {
		degrees += (int(message) + 1) * 360
	}

	self.Plan = fmt.Sprintf("t %d %d %d", self.Id, speed, degrees)
}

func (self *Pilot) PlanDock(planet hal.Planet) {
	self.Plan = fmt.Sprintf("d %d %d", self.Id, planet.Id)
	self.Overmind.NotifyDock(planet)
}

func (self *Pilot) PlanUndock() {
	self.Plan = fmt.Sprintf("u %d", self.Id)
}

// ----------------------------------------------

func (self *Pilot) ExecutePlan() {
	if self.Plan == "" {
		self.PlanThrust(0, 0, MSG_EXECUTED_NO_PLAN)
	}
	self.Game.RawOrder(self.Id, self.Plan)
	self.HasExecuted = true
}

func (self *Pilot) ExecutePlanIfStationary() {
	speed, _ := hal.CourseFromString(self.Plan)
	if speed == 0 {
		self.ExecutePlan()
	}
}

func (self *Pilot) ExecutePlanWithATC(atc *atc.AirTrafficControl) bool {

	speed, degrees := hal.CourseFromString(self.Plan)
	atc.Unrestrict(self.Ship, 0, 0)							// Unrestruct our preliminary null course so it doesn't block us.

	if atc.PathIsFree(self.Ship, speed, degrees) {

		self.ExecutePlan()
		atc.Restrict(self.Ship, speed, degrees)
		return true

	} else {

		atc.Restrict(self.Ship, 0, 0)						// Restrict our null course again.
		return false

	}
}

func (self *Pilot) SlowPlanDown() {

	speed, degrees := hal.CourseFromString(self.Plan)

	if speed < 1 {
		return
	}

	speed--

	self.PlanThrust(speed, degrees, MSG_ATC_SLOWED)
}
