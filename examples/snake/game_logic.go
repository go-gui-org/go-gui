// Game logic for the snake example, kept separate so the rules can
// be tested without the GUI.
package main

import (
	"math/rand"
	"slices"
	"time"
)

// Point is a cell coordinate on the game grid.
type Point struct {
	X int
	Y int
}

// Direction is the movement heading for the snake.
type Direction uint8

const (
	DirUp Direction = iota
	DirRight
	DirDown
	DirLeft
)

func (d Direction) delta() (dx, dy int) {
	switch d {
	case DirUp:
		return 0, -1
	case DirRight:
		return 1, 0
	case DirDown:
		return 0, 1
	case DirLeft:
		return -1, 0
	default:
		return 0, 0
	}
}

func isOppositeDirection(a, b Direction) bool {
	return (a == DirUp && b == DirDown) ||
		(a == DirDown && b == DirUp) ||
		(a == DirLeft && b == DirRight) ||
		(a == DirRight && b == DirLeft)
}

// IntnSource is the minimal random interface used for deterministic tests.
type IntnSource interface {
	Intn(n int) int
}

// Game holds all mutable snake game state.
type Game struct {
	rng IntnSource

	Snake []Point // head-first order
	Food  Point

	Width  int
	Height int

	Score int

	Direction Direction
	queuedDir Direction

	GameOver bool
	Paused   bool
	Won      bool
}

// NewGame returns a ready-to-play snake game.
func NewGame(width, height int, rng IntnSource) *Game {
	if width < 4 {
		width = 4
	}
	if height < 4 {
		height = 4
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	g := &Game{Width: width, Height: height, rng: rng}
	g.Reset()
	return g
}

// Reset restarts the game to its initial state.
func (g *Game) Reset() {
	cx := g.Width / 2
	cy := g.Height / 2
	g.Snake = []Point{{X: cx, Y: cy}, {X: cx - 1, Y: cy}, {X: cx - 2, Y: cy}}
	g.Direction = DirRight
	g.queuedDir = DirRight
	g.Score = 0
	g.GameOver = false
	g.Paused = false
	g.Won = false
	g.spawnFood()
}

// SetDirection queues a heading change for the next tick.
func (g *Game) SetDirection(next Direction) bool {
	if g.GameOver {
		return false
	}
	if isOppositeDirection(g.Direction, next) {
		return false
	}
	g.queuedDir = next
	return true
}

// TogglePause pauses/unpauses the game.
func (g *Game) TogglePause() {
	if g.GameOver {
		return
	}
	g.Paused = !g.Paused
}

// Tick advances the game by one step.
func (g *Game) Tick() {
	if g.GameOver || g.Paused {
		return
	}

	g.Direction = g.queuedDir
	dx, dy := g.Direction.delta()
	head := g.Snake[0]
	nextHead := Point{X: head.X + dx, Y: head.Y + dy}

	// Wall hits and self-collisions both end the run immediately.
	if nextHead.X < 0 || nextHead.X >= g.Width ||
		nextHead.Y < 0 || nextHead.Y >= g.Height {
		g.GameOver = true
		return
	}

	willGrow := nextHead == g.Food
	body := g.Snake
	if !willGrow {
		body = body[:len(body)-1]
	}
	if containsPoint(body, nextHead) {
		g.GameOver = true
		return
	}

	newSnake := make([]Point, 0, len(g.Snake)+1)
	newSnake = append(newSnake, nextHead)
	newSnake = append(newSnake, g.Snake...)
	if !willGrow {
		newSnake = newSnake[:len(newSnake)-1]
	} else {
		g.Score++
	}
	g.Snake = newSnake

	if willGrow {
		g.spawnFood()
	}
}

func (g *Game) spawnFood() {
	free := make([]Point, 0, g.Width*g.Height-len(g.Snake))
	occupied := make(map[Point]struct{}, len(g.Snake))
	for _, part := range g.Snake {
		occupied[part] = struct{}{}
	}
	for y := range g.Height {
		for x := range g.Width {
			p := Point{X: x, Y: y}
			if _, ok := occupied[p]; !ok {
				free = append(free, p)
			}
		}
	}

	if len(free) == 0 {
		g.Won = true
		g.GameOver = true
		g.Food = Point{X: -1, Y: -1}
		return
	}

	g.Food = free[g.rng.Intn(len(free))]
}

func containsPoint(points []Point, target Point) bool {
	return slices.Contains(points, target)
}
