package ai

import (
	"fmt"
	"math/rand"
	"sort"

	hal "../gohalite2"
)

type Pilot struct {
	hal.Ship
	Plan			string				// Our planned order, valid for 1 turn only
	HasExecuted		bool				// Have we actually sent the order?
	Overmind		*Overmind
	Game			*hal.Game
	TargetType		hal.EntityType		// NONE / SHIP / PLANET
	TargetId		int					// TargetId has no meaning if TargetType == NONE
}

func (self *Pilot) Log(format_string string, args ...interface{}) {
	format_string = fmt.Sprintf("%v: ", self) + format_string
	self.Game.Log(format_string, args...)
}

func (self *Pilot) Reset() {						// Doesn't clear Target
	self.Ship = self.Game.GetShip(self.Id)
	self.Plan = ""
	self.HasExecuted = false
	self.Game.RawOrder(self.Id, "")
}

func (self *Pilot) HasStationaryPlan() bool {		// true iff we DO have a plan, which doesn't move us.
	if self.Plan == "" {
		return false
	}
	speed, _ := hal.CourseFromString(self.Plan)
	return speed == 0
}

func (self *Pilot) ClosestPlanet() hal.Planet {
	return self.Game.ClosestPlanet(self)
}

func (self *Pilot) ValidateTarget() bool {

	game := self.Game

	switch self.TargetType {

	case hal.SHIP:

		target := game.GetShip(self.TargetId)
		if target.Alive() == false {
			self.TargetType = hal.NONE
		}

	case hal.PLANET:

		target := game.GetPlanet(self.TargetId)
		if target.Alive() == false {
			self.TargetType = hal.NONE
		} else if target.Owner == game.Pid() && target.IsFull() && len(self.Overmind.EnemyMap[target.Id]) == 0 {
			self.TargetType = hal.NONE
		}
	}

	if self.TargetType == hal.NONE {
		return false
	}

	return true
}

func (self *Pilot) PlanDockIfPossible() bool {
	if self.DockedStatus == hal.UNDOCKED {
		closest_planet := self.ClosestPlanet()
		if self.CanDock(closest_planet) {
			self.PlanDock(closest_planet)
			return true
		}
	}
	return false
}

func (self *Pilot) ChooseTarget() {
	game := self.Game

	all_planets := game.AllPlanets()
	var target_planets []hal.Planet

	for _, planet := range all_planets {

		ok := false

		if planet.Owner != game.Pid() {
			ok = true
		} else if planet.IsFull() == false {
			ok = true
		} else if len(self.Overmind.EnemyMap[planet.Id]) > len(self.Overmind.FriendlyMap[planet.Id]) {
			ok = true
		}

		if ok {
			target_planets = append(target_planets, planet)
		}
	}

	sort.Slice(target_planets, func(a, b int) bool {
		return self.Dist(target_planets[a]) < self.Dist(target_planets[b])
	})

	if len(target_planets) > 0 {
		self.TargetId = target_planets[0].Id
		self.TargetType = hal.PLANET
	}
}

func (self *Pilot) PlanChase(avoid_list []hal.Entity) {
	game := self.Game

	if self.DockedStatus != hal.UNDOCKED {
		return
	}

	switch self.TargetType {

	case hal.PLANET:

		planet := game.GetPlanet(self.TargetId)

		if self.ApproachDist(planet) < 4 {
			self.EngagePlanet(avoid_list)
			return
		}

		speed, degrees, err := game.GetApproach(self.Ship, planet, 4, avoid_list)

		if err != nil {
			self.Log("PlanChase(): %v", err)
			self.TargetType = hal.NONE
		} else {
			self.PlanThrust(speed, degrees)
		}

	case hal.SHIP:

		other_ship := game.GetShip(self.TargetId)

		speed, degrees, err := game.GetApproach(self.Ship, other_ship, 4.5, avoid_list)		// GetApproach uses centre-to-edge distances, so 4.5

		if err != nil {
			self.Log("PlanChase(): %v", err)
			self.TargetType = hal.NONE
		} else {
			self.PlanThrust(speed, degrees)
			if speed == 0 && self.Dist(other_ship) >= hal.WEAPON_RANGE {
				self.Log("PlanChase(): not moving but not in range!")
			}
		}

	case hal.NONE:

		self.PlanThrust(0, 0)

	}

}

func (self *Pilot) EngagePlanet(avoid_list []hal.Entity) {
	game := self.Game
	overmind := self.Overmind

	// We are very close to our target planet. Do something about this.

	if self.TargetType != hal.PLANET {
		self.Log("EngagePlanet() called but target wasn't a planet.")
		return
	}

	planet := game.GetPlanet(self.TargetId)

	// Is it friendly but under seige?

	if planet.Owned && planet.Owner == game.Pid() && len(overmind.EnemyMap[planet.Id]) > 0 {

		// We directly plan our move without changing our stored (planet) target.

		sort.Slice(overmind.EnemyMap[planet.Id], func(a, b int) bool {		// Sorting the list in place. Hmm.
			return overmind.EnemyMap[planet.Id][a].Dist(self.Ship) < overmind.EnemyMap[planet.Id][b].Dist(self.Ship)
		})

		speed, degrees, err := game.GetApproach(self.Ship, overmind.EnemyMap[planet.Id][0], 4.5, avoid_list)
		if err != nil {
			self.Log("EngagePlanet(), while trying to engage siege ship: %v", err)
		}
		self.PlanThrust(speed, degrees)
		return
	}

	// Is it available for us to dock?

	if planet.Owned == false || (planet.Owner == game.Pid() && planet.IsFull() == false) {
		self.FinalPlanetApproachForDock(avoid_list)
		return
	}

	// So it's hostile...

	if planet.Owner == game.Pid() {
		self.Log("EngagePlanet(): entered attack mode at friendly planet not under siege!")
	}

	docked_targets := game.ShipsDockedAt(planet)

	enemy_ship := docked_targets[rand.Intn(len(docked_targets))]
	self.TargetType = hal.SHIP
	self.TargetId = enemy_ship.Id

	speed, degrees, err := game.GetApproach(self.Ship, enemy_ship, 4.5, avoid_list)			// GetApproach uses centre-to-edge distances, so 4.5

	if err != nil {
		self.Log("EngagePlanet(): %v", err)
		return
	}

	self.PlanThrust(speed, degrees)
}

func (self *Pilot) FinalPlanetApproachForDock(avoid_list []hal.Entity) {
	game := self.Game

	if self.TargetType != hal.PLANET {
		self.Log("FinalPlanetApproachForDock() called but target wasn't a planet.", self.Id)
		return
	}

	planet := game.GetPlanet(self.TargetId)

	if self.CanDock(planet) {
		self.PlanDock(planet)
		return
	}

	speed, degrees, err := game.GetApproach(self.Ship, planet, 4, avoid_list)

	if err != nil {
		self.Log("FinalPlanetApproachForDock(): %v", self.Id, err)
	}

	self.PlanThrust(speed, degrees)
}

// -------------------------------------------------------------------

func (self *Pilot) PlanThrust(speed, degrees int) {

	for degrees < 0 {
		degrees += 360
	}

	degrees %= 360

	// We put some extra info into the angle, which we can see in the Chlorine replayer...

	var message int = -1

	if self.TargetType == hal.PLANET {
		message = self.TargetId
	} else if self.TargetType == hal.SHIP {
		message = 121
	} else if self.TargetType == hal.NONE {
		message = 180
	}

	if message > -1 {
		degrees += (message + 1) * 360
	}

	self.Plan = fmt.Sprintf("t %d %d %d", self.Id, speed, degrees)
}

func (self *Pilot) PlanDock(planet hal.Planet) {
	self.Plan = fmt.Sprintf("d %d %d", self.Id, planet.Id)
}

func (self *Pilot) PlanUndock() {
	self.Plan = fmt.Sprintf("u %d", self.Id)
}

// ----------------------------------------------

func (self *Pilot) ExecutePlan() {
	self.Game.RawOrder(self.Id, self.Plan)
	self.HasExecuted = true
}

func (self *Pilot) ExecutePlanIfStationary() {
	speed, _ := hal.CourseFromString(self.Plan)
	if speed == 0 {
		self.ExecutePlan()
	}
}

func (self *Pilot) ExecutePlanWithATC(atc *AirTrafficControl) bool {

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
