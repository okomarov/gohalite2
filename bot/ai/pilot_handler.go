package ai

import (
	"sort"

	hal "../core"
	pil "../pilot"
)

func (self *Overmind) ChooseTarget(pilot *pil.Pilot, all_planets []hal.Planet, all_enemy_ships []hal.Ship) {

	// We pass all_planets and all_enemy_ships for speed. They may get sorted in place, caller beware.

	if self.Game.InitialPlayers() == 2 {
		self.ChooseTarget2(pilot, all_planets, all_enemy_ships)
	} else {
		self.ChooseTarget4(pilot, all_planets, all_enemy_ships)
	}
}

func (self *Overmind) ChooseTarget2(pilot *pil.Pilot, all_planets []hal.Planet, all_enemy_ships []hal.Ship) {

	type TargetPlanet struct {
		planet					hal.Planet
		score					float64
	}

	game := self.Game

	var target_planets []TargetPlanet

	for _, planet := range all_planets {					// Could use ApproachDist in the things below?

		// It's always valid to go to threatened / enemy planets...

		if len(game.EnemiesNearPlanet(planet)) > 0 {
			target_planets = append(target_planets, TargetPlanet{planet, 1 / pilot.Dist(planet)})
			continue
		}

		// We can go to neutral or friendly planet sometimes...
		// (The planet is known to be one or the other, since no enemies were near it.)

		if game.DesiredSpots(planet) > 0 {
			commitment := len(self.PlanetChasers[planet.Id])
			if commitment < game.DesiredSpots(planet) {
				target_planets = append(target_planets, TargetPlanet{planet, 1 / (pilot.Dist(planet) * 1.4)})		// Lower score for these.
				continue
			}
		}
	}

	sort.Slice(target_planets, func(a, b int) bool {
		return target_planets[a].score > target_planets[b].score		// Note the reversal of the sort here
	})

	sort.Slice(all_enemy_ships, func(a, b int) bool {
		return pilot.Dist(all_enemy_ships[a]) < pilot.Dist(all_enemy_ships[b])
	})

	if len(all_enemy_ships) > 0 && len(target_planets) > 0 {
		if pilot.Dist(all_enemy_ships[0]) < pilot.Dist(target_planets[0].planet) {
			if self.ShipsChasing(all_enemy_ships[0]) == 0 {
				pilot.SetTarget(all_enemy_ships[0])
			} else {
				pilot.SetTarget(target_planets[0].planet)
			}
		} else {
			pilot.SetTarget(target_planets[0].planet)
		}
	} else if len(target_planets) > 0 {
		pilot.SetTarget(target_planets[0].planet)
	} else if len(all_enemy_ships) > 0 {
		pilot.SetTarget(all_enemy_ships[0])
	}
}

func (self *Overmind) ChooseTarget4(pilot *pil.Pilot, all_planets []hal.Planet, all_enemy_ships []hal.Ship) {

	game := self.Game

	var target_planets []hal.Planet

	for _, planet := range all_planets {

		ok := false

		// It's always valid to go to threatened / enemy planets...

		if len(game.EnemiesNearPlanet(planet)) > 0 {
			ok = true
		}

		// We can go to neutral or friendly planet sometimes...

		if game.DesiredSpots(planet) > 0 {
			commitment := len(self.PlanetChasers[planet.Id])
			if commitment < game.DesiredSpots(planet) {
				ok = true
			}
		}

		if ok {
			target_planets = append(target_planets, planet)
		}
	}

	sort.Slice(target_planets, func(a, b int) bool {
		return pilot.Dist(target_planets[a]) < pilot.Dist(target_planets[b])	// Could use ApproachDist
	})

	sort.Slice(all_enemy_ships, func(a, b int) bool {
		return pilot.Dist(all_enemy_ships[a]) < pilot.Dist(all_enemy_ships[b])
	})

	if len(all_enemy_ships) > 0 && len(target_planets) > 0 {
		if pilot.Dist(all_enemy_ships[0]) < pilot.Dist(target_planets[0]) {
			if self.ShipsChasing(all_enemy_ships[0]) == 0 {
				pilot.SetTarget(all_enemy_ships[0])
			} else {
				pilot.SetTarget(target_planets[0])
			}
		} else {
			pilot.SetTarget(target_planets[0])
		}
	} else if len(target_planets) > 0 {
		pilot.SetTarget(target_planets[0])
	} else if len(all_enemy_ships) > 0 {
		pilot.SetTarget(all_enemy_ships[0])
	}
}

func (self *Overmind) ValidateTarget(pilot *pil.Pilot) bool {

	game := self.Game

	switch pilot.Target.Type() {

	case hal.SHIP:

		if pilot.Target.Alive() == false {
			pilot.SetTarget(hal.Nothing{})
		}

	case hal.PLANET:

		target := pilot.Target.(hal.Planet)

		if target.Alive() == false {
			pilot.SetTarget(hal.Nothing{})
		} else if self.ShipsAboutToDock(target) >= game.DesiredSpots(target) {		// We've enough guys (maybe 0) trying to dock...
			if len(game.EnemiesNearPlanet(target)) == 0 {							// ...and the planet is safe
				pilot.SetTarget(hal.Nothing{})
			}
		}
	}

	if pilot.Target == (hal.Nothing{}) {
		return false
	}

	return true
}

func (self *Overmind) DockIfWise(pilot *pil.Pilot) bool {

	if pilot.DockedStatus != hal.UNDOCKED {
		return false
	}

	closest_planet := pilot.ClosestPlanet()

	if pilot.CanDock(closest_planet) == false {
		return false
	}

	// Pilots with point targets should always succeed in docking...

	if pilot.Target.Type() == hal.POINT {
		pilot.SetTarget(closest_planet)			// It would be sad to stay with a Point target forever...
		pilot.PlanDock(closest_planet)
		return true
	}

	// Otherwise we check some things...

	if len(self.Game.EnemiesNearPlanet(closest_planet)) > 0 {
		return false
	}

	if self.ShipsAboutToDock(closest_planet) >= self.Game.DesiredSpots(closest_planet) {
		return false
	}

	pilot.PlanDock(closest_planet)
	return true
}

