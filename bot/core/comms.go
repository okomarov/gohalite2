package core

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------

type TokenParser struct {
	scanner		*bufio.Scanner
	count		int
	all_tokens	[]string		// This is used for logging only. It is cleared each time it's asked-for.
}

func NewTokenParser() *TokenParser {
	ret := new(TokenParser)
	ret.scanner = bufio.NewScanner(os.Stdin)
	ret.scanner.Split(bufio.ScanWords)
	return ret
}

func (self *TokenParser) Int() int {
	bl := self.scanner.Scan()
	if bl == false {
		err := self.scanner.Err()
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		} else {
			panic(fmt.Sprintf("End of input."))
		}
	}
	self.all_tokens = append(self.all_tokens, self.scanner.Text())
	ret, err := strconv.Atoi(self.scanner.Text())
	if err != nil {
		panic(fmt.Sprintf("TokenReader.Int(): Atoi failed at token %d: \"%s\"", self.count, self.scanner.Text()))
	}
	self.count++
	return ret
}

func (self *TokenParser) DockedStatus() DockedStatus {
	return DockedStatus(self.Int())
}

func (self *TokenParser) Float() float64 {
	bl := self.scanner.Scan()
	if bl == false {
		err := self.scanner.Err()
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		} else {
			panic(fmt.Sprintf("End of input."))
		}
	}
	self.all_tokens = append(self.all_tokens, self.scanner.Text())
	ret, err := strconv.ParseFloat(self.scanner.Text(), 64)
	if err != nil {
		panic(fmt.Sprintf("TokenReader.Float(): ParseFloat failed at token %d: \"%s\"", self.count, self.scanner.Text()))
	}
	self.count++
	return ret
}

func (self *TokenParser) Bool() bool {
	val := self.Int()
	if val != 0 && val != 1 {
		panic(fmt.Sprintf("TokenReader.Bool(): Value wasn't 0 or 1 (was: \"%d\")", val))
	}
	return val == 1
}

func (self *TokenParser) Tokens(sep string) string {
	ret := strings.Join(self.all_tokens, sep)
	self.all_tokens = nil
	return ret
}

func (self *TokenParser) ClearTokens() {
	self.all_tokens = nil
}

// ---------------------------------------

func (self *Game) Parse() {

	// Do our first read before clearing things, so that it panics on EOF and we haven't corrupted our state...

	player_count := self.token_parser.Int()

	// Now reset various things...

	self.orders = make(map[int]string)			// Clear all orders.
	self.messages = make(map[int]int)

	if self.inited {
		self.turn++
	}

	// We are about to remake the ship and planet maps, but we do need to keep the old ones for a bit...

	old_shipmap := self.shipMap
	old_planetmap := self.planetMap

	// Set everything to have 0 health.
	// Thus it shall be seen as dead by anything with references to it, unless updated below...

	for _, ship := range old_shipmap {
		ship.HP = 0
	}

	for _, planet := range old_planetmap {
		planet.HP = 0
	}

	// Clear some info maps. We will recreate them during parsing.

	self.shipMap = make(map[int]*Ship)
	self.planetMap = make(map[int]*Planet)
	self.dockMap = make(map[int][]*Ship)
	self.playershipMap = make(map[int][]*Ship)
	self.playerplanetMap = make(map[int][]*Planet)

	// Player parsing.............................................................................

	self.parse_time = time.Now()				// MUST happen AFTER the first token parse. <------------------------------------- important

	if self.initialPlayers == 0 {
		self.initialPlayers = player_count		// Only save this at init stage.
	}

	players_with_ships := 0

	for p := 0; p < player_count; p++ {

		pid := self.token_parser.Int()

		ship_count := self.token_parser.Int()

		if ship_count > 0 {
			players_with_ships++
		}

		for s := 0; s < ship_count; s++ {

			sid := self.token_parser.Int()

			ship, ok := old_shipmap[sid]

			// Save the previous state of the ship, if any...

			var last_ship *Ship
			if ok {
				last_ship = new(Ship);
				*last_ship = *ship
			} else {
				ship = new(Ship)
				last_ship = nil
			}

			ship.Id = sid
			ship.Owner = pid

			ship.X = self.token_parser.Float()
			ship.Y = self.token_parser.Float()
			ship.HP = self.token_parser.Int()
			self.token_parser.Float()								// Skip deprecated "speedx"
			self.token_parser.Float()								// Skip deprecated "speedy"
			ship.DockedStatus = self.token_parser.DockedStatus()
			ship.DockedPlanet = self.token_parser.Int()
			ship.DockingProgress = self.token_parser.Int()

			ship.fudge_dock_status()								// Correct for Halite's oddity about docking status

			if ship.DockedStatus == UNDOCKED {
				ship.DockedPlanet = -1
			}

			self.token_parser.Int()									// Skip deprecated "cooldown"

			if last_ship == nil {
				ship.Birth = Max(0, self.turn)						// Turn can be -1 in init stage.
				ship.SpawnX = ship.X
				ship.SpawnY = ship.Y
				self.cumulativeShips[pid]++
			} else {
				ship.Dx = ship.X - last_ship.X
				ship.Dy = ship.Y - last_ship.Y
				ship.LastSpeed = Round(math.Sqrt(ship.Dx * ship.Dx + ship.Dy * ship.Dy))
				ship.LastAngle = Angle(last_ship.X, last_ship.Y, ship.X, ship.Y)
			}

			// Some inferred tactical info that we will set later...

			if ship.Doomed {
				self.Log("Ship %d survived previous turn despite us predicting its doom.", ship.Id)
			}

			ship.Firing = false
			ship.Doomed = false

			// Add the ship to our maps (if needed)...

			self.shipMap[sid] = ship
			self.playershipMap[pid] = append(self.playershipMap[pid], ship)
		}

		sort.Slice(self.playershipMap[pid], func(a, b int) bool {
			return self.playershipMap[pid][a].Id < self.playershipMap[pid][b].Id
		})
	}

	// Planet parsing.............................................................................

	planet_count := self.token_parser.Int()

	for p := 0; p < planet_count; p++ {

		plid := self.token_parser.Int()

		planet, ok := old_planetmap[plid]
		if ok == false {
			planet = new(Planet)
		}

		planet.Id = plid

		planet.X = self.token_parser.Float()
		planet.Y = self.token_parser.Float()
		planet.HP = self.token_parser.Int()
		planet.Radius = self.token_parser.Float()
		planet.DockingSpots = self.token_parser.Int()
		planet.CurrentProduction = self.token_parser.Int()
		self.token_parser.Int()										// Skip deprecated "remaining production"
		planet.Owned = self.token_parser.Bool()
		planet.Owner = self.token_parser.Int()

		if planet.Owned == false {
			planet.Owner = -1
		} else {
			self.lastownerMap[planet.Id] = planet.Owner
		}

		planet.DockedShips = self.token_parser.Int()
		official_docked_ships := planet.DockedShips					// How many ships the engine will send us, regardless of truth...

		// The dockMap is kept separately due to an old design decision...
		// i.e. the Planet struct itself does not get the following data:

		for s := 0; s < official_docked_ships; s++ {

			// This relies on the fact that we've already been given info about the ships...

			sid := self.token_parser.Int()
			ship, ok := self.GetShip(sid)
			if ok == false {
				panic("Parser choked on GetShip(sid)")
			}

			if ship.DockedStatus != UNDOCKED {								// Can be false due to fudge_dock_status()
				self.dockMap[plid] = append(self.dockMap[plid], ship)
			} else {
				planet.DockedShips--										// Also needed due to fudge_dock_status()
			}
		}

		if planet.DockedShips == 0 {										// Also needed due to fudge_dock_status()
			planet.Owner = -1
			planet.Owned = false
		}

		sort.Slice(self.dockMap[plid], func(a, b int) bool {
			return self.dockMap[plid][a].Id < self.dockMap[plid][b].Id
		})

		self.planetMap[plid] = planet
		self.playerplanetMap[planet.Owner] = append(self.playerplanetMap[planet.Owner], planet)		// This is fine if Owner == -1
	}

	for pid, _ := range self.playerplanetMap {
		sort.Slice(self.playerplanetMap[pid], func(a, b int) bool {
			return self.playerplanetMap[pid][a].Id < self.playerplanetMap[pid][b].Id
		})
	}

	// Query responses (see info.go)... while these could be done interleaved with the above, they are separated for clarity.

	self.all_ships_cache = nil
	for _, ship := range self.shipMap {
		self.all_ships_cache = append(self.all_ships_cache, ship)
	}
	sort.Slice(self.all_ships_cache, func(a, b int) bool {
		return self.all_ships_cache[a].Id < self.all_ships_cache[b].Id
	})

	self.enemy_ships_cache = nil
	for _, ship := range self.shipMap {
		if ship.Owner != self.pid {
			self.enemy_ships_cache = append(self.enemy_ships_cache, ship)
		}
	}
	sort.Slice(self.enemy_ships_cache, func(a, b int) bool {
		return self.enemy_ships_cache[a].Id < self.enemy_ships_cache[b].Id
	})

	self.all_planets_cache = nil
	for _, planet := range self.planetMap {
		self.all_planets_cache = append(self.all_planets_cache, planet)
	}
	sort.Slice(self.all_planets_cache, func(a, b int) bool {
		return self.all_planets_cache[a].Id < self.all_planets_cache[b].Id
	})

	self.all_immobile_cache = nil
	for _, planet := range self.planetMap {
		self.all_immobile_cache = append(self.all_immobile_cache, planet)
		for _, ship := range self.ShipsDockedAt(planet) {
			self.all_immobile_cache = append(self.all_immobile_cache, ship)
		}
	}
	sort.Slice(self.all_immobile_cache, func(a, b int) bool {
		if self.all_immobile_cache[a].Type() == PLANET && self.all_immobile_cache[b].Type() == SHIP {
			return true
		}
		if self.all_immobile_cache[a].Type() == SHIP && self.all_immobile_cache[b].Type() == PLANET {
			return false
		}
		return self.all_immobile_cache[a].GetId() < self.all_immobile_cache[b].GetId()
	})

	// Some meta info...

	self.currentPlayers = players_with_ships

	old_raw := self.raw
	self.raw = self.token_parser.Tokens(" ")
	if old_raw == self.raw {
		self.run_of_sames++
	} else {
		self.run_of_sames = 0
	}

	self.UpdateEnemyMaps()
	self.UpdateFriendMap()
	self.PredictTimeZero()
	self.UpdateShipNearestEnemies()
}

// ---------------------------------------

func (self *Game) Thrust(ship *Ship, speed, degrees int) {
	for degrees < 0 { degrees += 360 }; degrees %= 360
	self.orders[ship.Id] = fmt.Sprintf("t %d %d %d", ship.Id, speed, degrees)
}

func (self *Game) SetMessage(ship *Ship, message int) {
	if message < 0 || message > 180 {
		return
	}
	self.messages[ship.Id] = message
}

func (self *Game) Dock(ship *Ship, planet Planet) {
	self.orders[ship.Id] = fmt.Sprintf("d %d %d", ship.Id, planet.Id)
}

func (self *Game) Undock(ship *Ship) {
	self.orders[ship.Id] = fmt.Sprintf("u %d", ship.Id)
}

func (self *Game) ClearOrder(ship *Ship) {
	delete(self.orders, ship.Id)
}

func (self *Game) CurrentOrder(ship *Ship) string {
	return self.orders[ship.Id]
}

func (self *Game) RawOrder(sid int, s string) {
	self.orders[sid] = s
}

func (self *Game) Send(no_messages bool) {
	fmt.Printf(self.RawOutput(false, no_messages))
	fmt.Printf("\n")
}
