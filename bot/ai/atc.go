package ai

import (
	hal "../gohalite2"
)

const (
	TIME_STEPS = 7
	RESOLUTION = 2		// i.e. double resolution
	FILL_RADIUS = 1		// filling from -1 to +1 inclusive
)

type ATC struct {
	Grid	[][][]bool
	Width	int
	Height	int
}

func NewATC(world_width, world_height int) *ATC {

	ret := new(ATC)

	ret.Width = world_width * RESOLUTION
	ret.Height = world_height * RESOLUTION

	ret.Grid = make([][][]bool, ret.Width)

	for x := 0; x < ret.Width; x++ {
		ret.Grid[x] = make([][]bool, ret.Height)
		for y := 0; y < ret.Height; y++ {
			ret.Grid[x][y] = make([]bool, TIME_STEPS)
		}
	}

	return ret
}

func (self *ATC) Clear() {
	for x := 0; x < self.Width; x++ {
		for y := 0; y < self.Height; y++ {
			for t := 0; t < TIME_STEPS; t++ {
				self.Grid[x][y][t] = false
			}
		}
	}
}

func (self *ATC) Claim(ship hal.Ship, speed, degrees int) {

	x2, y2 := hal.Projection(ship.X, ship.Y, float64(speed), degrees)

	stepx := (x2 - ship.X) / TIME_STEPS
	stepy := (y2 - ship.Y) / TIME_STEPS

	x := ship.X
	y := ship.Y

	for t := 0; t < TIME_STEPS; t++ {

		x += stepx
		y += stepy

		grid_x := int(x) * RESOLUTION
		grid_y := int(y) * RESOLUTION

		for index_x := grid_x - FILL_RADIUS; index_x <= grid_x + FILL_RADIUS; index_x++ {
			for index_y := grid_y - FILL_RADIUS; index_y <= grid_y + FILL_RADIUS; index_y++ {
				if index_x >= 0 && index_x < self.Width && index_y >= 0 && index_y < self.Height {
					self.Grid[index_x][index_y][t] = true
				}
			}
		}
	}
}

func (self *ATC) PathIsFree(ship hal.Ship, speed, degrees int) bool {

	x2, y2 := hal.Projection(ship.X, ship.Y, float64(speed), degrees)

	stepx := (x2 - ship.X) / TIME_STEPS
	stepy := (y2 - ship.Y) / TIME_STEPS

	x := ship.X
	y := ship.Y

	for t := 0; t < TIME_STEPS; t++ {

		x += stepx
		y += stepy

		grid_x := int(x) * RESOLUTION
		grid_y := int(y) * RESOLUTION

		for index_x := grid_x - FILL_RADIUS; index_x <= grid_x + FILL_RADIUS; index_x++ {
			for index_y := grid_y - FILL_RADIUS; index_y <= grid_y + FILL_RADIUS; index_y++ {
				if index_x >= 0 && index_x < self.Width && index_y >= 0 && index_y < self.Height {
					if self.Grid[index_x][index_y][t] {
						return false
					}
				}
			}
		}
	}

	return true
}
