package gohalite2

// ----------------------------------------------

func (self *Game) GetShip(sid int) Ship {
	return self.shipMap[sid]
}

func (self *Game) GetPlanet(plid int) Planet {
	return self.planetMap[plid]
}

// ----------------------------------------------

func (self *Game) AllShips() []Ship {
	var ret []Ship
	for sid, _ := range self.shipMap {
		ship := self.GetShip(sid)
		if ship.Alive() {
			ret = append(ret, ship)
		}
	}
	return ret
}

func (self *Game) AllPlanets() []Planet {
	var ret []Planet
	for plid, _ := range self.planetMap {
		planet := self.GetPlanet(plid)
		if planet.Alive() {
			ret = append(ret, planet)
		}
	}
	return ret
}

func (self *Game) AllImmobile() []Entity {		// Returns all planets and all docked ships
	var ret []Entity
	for plid, _ := range self.planetMap {
		planet := self.GetPlanet(plid)
		if planet.Alive() {
			ret = append(ret, planet)
			all_docked := self.ShipsDockedAt(planet)
			for _, s := range all_docked {
				ret = append(ret, s)
			}
		}
	}
	return ret
}

// ----------------------------------------------

func (self *Game) PlanetsOwnedBy(pid int) []Planet {
	var ret []Planet
	for plid, _ := range self.planetMap {
		planet := self.GetPlanet(plid)
		if planet.Alive() && planet.Owned && planet.Owner == pid {
			ret = append(ret, planet)
		}
	}
	return ret
}

func (self *Game) ShipsOwnedBy(pid int) []Ship {
	var ret []Ship
	for sid, _ := range self.shipMap {
		ship := self.GetShip(sid)
		if ship.Alive() && ship.Owner == pid {
			ret = append(ret, ship)
		}
	}
	return ret
}

func (self *Game) MyPlanets() []Planet {
	return self.PlanetsOwnedBy(self.pid)
}

func (self *Game) MyShips() []Ship {
	return self.ShipsOwnedBy(self.pid)
}

// ----------------------------------------------

func (self *Game) MyNewShipIDs() []int {			// My ships born this turn.
	var ret []int
	for sid, _ := range self.shipMap {
		ship := self.shipMap[sid]
		if ship.Birth == self.turn && ship.Owner == self.pid && ship.Alive() {
			ret = append(ret, ship.Id)
		}
	}
	return ret
}

func (self *Game) ShipsDockedAt(pl Planet) []Ship {

	var ret []Ship

	for _, Ship := range self.dockMap[pl.Id] {
		ret = append(ret, Ship)
	}

	return ret
}

func (self *Game) ClosestPlanet(e Entity) Planet {

	var best_dist float64 = 9999999
	var ret Planet

	for _, planet := range self.AllPlanets() {
		dist := e.ApproachDist(planet)
		if dist < best_dist {
			best_dist = dist
			ret = planet
		}
	}

	return ret
}