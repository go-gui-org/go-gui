// Game logic for Minesweeper, kept separate for headless testing.
package main

import "math/rand/v2"

type CellState uint8

const (
	CellHidden CellState = iota
	CellRevealed
	CellFlagged
)

type GameState uint8

const (
	GamePlaying GameState = iota
	GameWon
	GameLost
)

type Difficulty uint8

const (
	DiffBeginner Difficulty = iota
	DiffIntermediate
	DiffExpert
)

func (d Difficulty) Config() (rows, cols, mines int) {
	switch d {
	case DiffIntermediate:
		return 16, 16, 40
	case DiffExpert:
		return 16, 30, 99
	default:
		return 9, 9, 10
	}
}

type Point struct{ Row, Col int }

type Cell struct {
	Adjacent int
	Mine     bool
	State    CellState
}

// RandSource abstracts random number generation for deterministic
// tests.
type RandSource interface {
	IntN(n int) int
}

type Game struct {
	rng        RandSource
	Board      []Cell
	Rows       int
	Cols       int
	MineCount  int
	FlagsUsed  int
	Revealed   int
	State      GameState
	FirstClick bool
	NoGuess    bool
}

func NewGame(rows, cols, mines int, noGuess bool, rng RandSource) *Game {
	if rng == nil {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	g := &Game{
		Rows:      rows,
		Cols:      cols,
		MineCount: mines,
		NoGuess:   noGuess,
		rng:       rng,
	}
	g.Reset()
	return g
}

func (g *Game) Reset() {
	g.Board = make([]Cell, g.Rows*g.Cols)
	g.State = GamePlaying
	g.FlagsUsed = 0
	g.Revealed = 0
	g.FirstClick = true
}

func (g *Game) idx(r, c int) int       { return r*g.Cols + c }
func (g *Game) inBounds(r, c int) bool { return r >= 0 && r < g.Rows && c >= 0 && c < g.Cols }
func (g *Game) cell(r, c int) *Cell    { return &g.Board[g.idx(r, c)] }

func (g *Game) neighbors(r, c int) []Point {
	pts := make([]Point, 0, 8)
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			nr, nc := r+dr, c+dc
			if g.inBounds(nr, nc) {
				pts = append(pts, Point{nr, nc})
			}
		}
	}
	return pts
}

func (g *Game) placeMines(safeRow, safeCol int) {
	if g.NoGuess {
		board := GenerateNoGuess(
			g.Rows, g.Cols, g.MineCount, safeRow, safeCol, g.rng)
		if board != nil {
			g.Board = board
			return
		}
	}
	g.placeMinesRandom(safeRow, safeCol)
}

func (g *Game) placeMinesRandom(safeRow, safeCol int) {
	total := g.Rows * g.Cols
	excluded := make(map[int]bool, 9)
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			nr, nc := safeRow+dr, safeCol+dc
			if g.inBounds(nr, nc) {
				excluded[g.idx(nr, nc)] = true
			}
		}
	}
	placed := 0
	for placed < g.MineCount {
		i := g.rng.IntN(total)
		if excluded[i] || g.Board[i].Mine {
			continue
		}
		g.Board[i].Mine = true
		placed++
	}
	g.computeAdjacent()
}

func (g *Game) computeAdjacent() {
	for r := range g.Rows {
		for c := range g.Cols {
			if g.Board[g.idx(r, c)].Mine {
				continue
			}
			count := 0
			for _, n := range g.neighbors(r, c) {
				if g.Board[g.idx(n.Row, n.Col)].Mine {
					count++
				}
			}
			g.Board[g.idx(r, c)].Adjacent = count
		}
	}
}

// Reveal opens a cell. Returns false if a mine was hit.
func (g *Game) Reveal(row, col int) bool {
	if g.State != GamePlaying || !g.inBounds(row, col) {
		return true
	}
	if g.cell(row, col).State != CellHidden {
		return true
	}
	if g.FirstClick {
		g.FirstClick = false
		g.placeMines(row, col)
		// Safe first click: cell is guaranteed not a mine.
		g.floodFill(row, col)
		g.checkWin()
		return true
	}
	if g.cell(row, col).Mine {
		g.cell(row, col).State = CellRevealed
		g.State = GameLost
		return false
	}
	g.floodFill(row, col)
	g.checkWin()
	return g.State != GameLost
}

func (g *Game) floodFill(row, col int) {
	queue := []Point{{row, col}}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		c := g.cell(p.Row, p.Col)
		if c.State != CellHidden || c.Mine {
			continue
		}
		c.State = CellRevealed
		g.Revealed++
		if c.Adjacent == 0 {
			for _, n := range g.neighbors(p.Row, p.Col) {
				if g.cell(n.Row, n.Col).State == CellHidden {
					queue = append(queue, n)
				}
			}
		}
	}
}

func (g *Game) Flag(row, col int) {
	if g.State != GamePlaying || !g.inBounds(row, col) {
		return
	}
	c := g.cell(row, col)
	switch c.State {
	case CellHidden:
		c.State = CellFlagged
		g.FlagsUsed++
	case CellFlagged:
		c.State = CellHidden
		g.FlagsUsed--
	}
}

// Chord reveals unflagged neighbors when flag count matches the
// cell number. Returns false if a mine was hit.
func (g *Game) Chord(row, col int) bool {
	if g.State != GamePlaying || !g.inBounds(row, col) {
		return true
	}
	c := g.cell(row, col)
	if c.State != CellRevealed || c.Adjacent == 0 {
		return true
	}
	flagCount := 0
	for _, n := range g.neighbors(row, col) {
		if g.cell(n.Row, n.Col).State == CellFlagged {
			flagCount++
		}
	}
	if flagCount != c.Adjacent {
		return true
	}
	for _, n := range g.neighbors(row, col) {
		nc := g.cell(n.Row, n.Col)
		if nc.State != CellHidden {
			continue
		}
		if nc.Mine {
			nc.State = CellRevealed
			g.State = GameLost
			return false
		}
		g.floodFill(n.Row, n.Col)
	}
	g.checkWin()
	return g.State != GameLost
}

func (g *Game) checkWin() {
	if g.Revealed == g.Rows*g.Cols-g.MineCount {
		g.State = GameWon
	}
}

// CheckBoard returns cells where flags exceed the adjacent mine count.
func (g *Game) CheckBoard() []Point {
	var bad []Point
	for r := range g.Rows {
		for c := range g.Cols {
			cell := g.cell(r, c)
			if cell.State != CellRevealed || cell.Adjacent == 0 {
				continue
			}
			flagCount := 0
			for _, n := range g.neighbors(r, c) {
				if g.cell(n.Row, n.Col).State == CellFlagged {
					flagCount++
				}
			}
			if flagCount > cell.Adjacent {
				bad = append(bad, Point{r, c})
			}
		}
	}
	return bad
}

// AutoFlagMines flags all remaining unflagged mines (used on win).
func (g *Game) AutoFlagMines() {
	for i := range g.Board {
		if g.Board[i].Mine && g.Board[i].State == CellHidden {
			g.Board[i].State = CellFlagged
			g.FlagsUsed++
		}
	}
}
