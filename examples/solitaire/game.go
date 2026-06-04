// Game logic for Klondike Solitaire, kept separate for headless
// testing.
package main

import (
	"math/rand/v2"
	"slices"
)

// Suit identifies a card's suit.
type Suit uint8

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

// IsRed reports whether the suit is red (Hearts or Diamonds).
func (s Suit) IsRed() bool { return s == Hearts || s == Diamonds }

// Symbol returns the Unicode symbol for the suit.
func (s Suit) Symbol() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	}
	return "?"
}

// Card is a single playing card.
type Card struct {
	Rank   int // 1=Ace, 2–10, 11=Jack, 12=Queen, 13=King
	Suit   Suit
	FaceUp bool
}

// RankString returns the display string for the card's rank.
func (c Card) RankString() string {
	switch c.Rank {
	case 1:
		return "A"
	case 10:
		return "10"
	case 11:
		return "J"
	case 12:
		return "Q"
	case 13:
		return "K"
	default:
		if c.Rank >= 2 && c.Rank <= 9 {
			return string(rune('0' + c.Rank))
		}
		return "?"
	}
}

// DrawMode controls how many cards are drawn from the stock.
type DrawMode uint8

const (
	DrawOne   DrawMode = 1
	DrawThree DrawMode = 3
)

// GameState tracks the current game phase.
type GameState uint8

const (
	StatePlaying GameState = iota
	StateWon
)

// SourceType identifies where a card came from.
type SourceType uint8

const (
	SourceTableau SourceType = iota
	SourceWaste
	SourceFoundation
)

// RandSource abstracts random number generation for
// deterministic tests.
type RandSource interface {
	IntN(n int) int
}

// Game holds the full state of a Klondike Solitaire game.
type Game struct {
	Tableau    [7][]Card
	Foundation [4][]Card // indexed by Suit
	Stock      []Card
	Waste      []Card
	DrawMode   DrawMode
	State      GameState
	Moves      int
	Score      int
}

// NewGame creates and deals a new game. If rng is nil a default
// source is used.
func NewGame(mode DrawMode, rng RandSource) *Game {
	g := &Game{DrawMode: mode}
	deck := newDeck()
	shuffle(deck, rng)
	deal(g, deck)
	return g
}

// Reset re-deals the game with a fresh shuffle.
func (g *Game) Reset(rng RandSource) {
	mode := g.DrawMode
	*g = Game{DrawMode: mode}
	deck := newDeck()
	shuffle(deck, rng)
	deal(g, deck)
}

func newDeck() []Card {
	deck := make([]Card, 0, 52)
	for s := Spades; s <= Clubs; s++ {
		for r := 1; r <= 13; r++ {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}
	return deck
}

func shuffle(deck []Card, rng RandSource) {
	if rng == nil {
		rng = defaultRNG{}
	}
	for i := len(deck) - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}

type defaultRNG struct{}

func (defaultRNG) IntN(n int) int { return rand.IntN(n) }

func deal(g *Game, deck []Card) {
	idx := 0
	for col := range 7 {
		g.Tableau[col] = make([]Card, col+1)
		for row := 0; row <= col; row++ {
			g.Tableau[col][row] = deck[idx]
			idx++
		}
		// Top card face-up.
		g.Tableau[col][col].FaceUp = true
	}
	// Remaining cards go to stock (face-down).
	g.Stock = make([]Card, len(deck)-idx)
	copy(g.Stock, deck[idx:])
}

// Draw moves cards from stock to waste. If stock is empty,
// recycles waste back to stock.
func (g *Game) Draw() {
	if len(g.Stock) == 0 {
		if len(g.Waste) == 0 {
			return
		}
		// Recycle: reverse waste back to stock.
		for i := range slices.Backward(g.Waste) {
			g.Waste[i].FaceUp = false
			g.Stock = append(g.Stock, g.Waste[i])
		}
		g.Waste = g.Waste[:0]
		if g.DrawMode == DrawThree {
			g.Score -= 100
		}
		return
	}

	n := min(int(g.DrawMode), len(g.Stock))
	// Draw from top of stock (end of slice).
	for range n {
		top := len(g.Stock) - 1
		c := g.Stock[top]
		c.FaceUp = true
		g.Stock = g.Stock[:top]
		g.Waste = append(g.Waste, c)
	}
}

// TopWaste returns the top waste card or nil.
func (g *Game) TopWaste() *Card {
	if len(g.Waste) == 0 {
		return nil
	}
	return &g.Waste[len(g.Waste)-1]
}

// VisibleWaste returns up to 3 top waste cards for fan display.
// Result is ordered bottom-to-top (last element is the top card).
func (g *Game) VisibleWaste() []Card {
	n := len(g.Waste)
	if n == 0 {
		return nil
	}
	start := max(n-3, 0)
	return g.Waste[start:]
}

// CanPlaceOnTableau checks if card can be placed on the given
// tableau column.
func (g *Game) CanPlaceOnTableau(c Card, col int) bool {
	pile := g.Tableau[col]
	if len(pile) == 0 {
		return c.Rank == 13 // Only kings on empty columns.
	}
	top := pile[len(pile)-1]
	return top.FaceUp &&
		c.Suit.IsRed() != top.Suit.IsRed() &&
		c.Rank == top.Rank-1
}

// CanPlaceOnFoundation checks if card can be placed on its
// foundation pile.
func (g *Game) CanPlaceOnFoundation(c Card) bool {
	pile := g.Foundation[c.Suit]
	if len(pile) == 0 {
		return c.Rank == 1 // Only aces on empty foundations.
	}
	top := pile[len(pile)-1]
	return c.Rank == top.Rank+1
}

// MoveWasteToTableau moves the top waste card to a tableau column.
func (g *Game) MoveWasteToTableau(col int) bool {
	tw := g.TopWaste()
	if tw == nil {
		return false
	}
	if !g.CanPlaceOnTableau(*tw, col) {
		return false
	}
	g.Tableau[col] = append(g.Tableau[col], *tw)
	g.Waste = g.Waste[:len(g.Waste)-1]
	g.Moves++
	g.Score += 5
	return true
}

// MoveWasteToFoundation moves the top waste card to its
// foundation pile.
func (g *Game) MoveWasteToFoundation() bool {
	tw := g.TopWaste()
	if tw == nil {
		return false
	}
	if !g.CanPlaceOnFoundation(*tw) {
		return false
	}
	g.Foundation[tw.Suit] = append(g.Foundation[tw.Suit], *tw)
	g.Waste = g.Waste[:len(g.Waste)-1]
	g.Moves++
	g.Score += 10
	g.checkWin()
	return true
}

// MoveTableauToTableau moves a stack of face-up cards from one
// tableau column to another. cardIdx is the index of the first
// card in the stack to move.
func (g *Game) MoveTableauToTableau(srcCol, cardIdx, dstCol int) bool {
	src := g.Tableau[srcCol]
	if cardIdx < 0 || cardIdx >= len(src) {
		return false
	}
	if !src[cardIdx].FaceUp {
		return false
	}
	if !g.CanPlaceOnTableau(src[cardIdx], dstCol) {
		return false
	}
	g.Tableau[dstCol] = append(g.Tableau[dstCol], src[cardIdx:]...)
	g.Tableau[srcCol] = src[:cardIdx]
	g.flipTopCard(srcCol)
	g.Moves++
	return true
}

// MoveTableauToFoundation moves the top card of a tableau column
// to its foundation pile.
func (g *Game) MoveTableauToFoundation(srcCol int) bool {
	pile := g.Tableau[srcCol]
	if len(pile) == 0 {
		return false
	}
	top := pile[len(pile)-1]
	if !top.FaceUp {
		return false
	}
	if !g.CanPlaceOnFoundation(top) {
		return false
	}
	g.Foundation[top.Suit] = append(g.Foundation[top.Suit], top)
	g.Tableau[srcCol] = pile[:len(pile)-1]
	g.flipTopCard(srcCol)
	g.Moves++
	g.Score += 10
	g.checkWin()
	return true
}

// MoveFoundationToTableau moves the top foundation card back to
// a tableau column.
func (g *Game) MoveFoundationToTableau(suit Suit, col int) bool {
	pile := g.Foundation[suit]
	if len(pile) == 0 {
		return false
	}
	top := pile[len(pile)-1]
	if !g.CanPlaceOnTableau(top, col) {
		return false
	}
	g.Tableau[col] = append(g.Tableau[col], top)
	g.Foundation[suit] = pile[:len(pile)-1]
	g.Moves++
	g.Score -= 15
	return true
}

// AutoMoveToFoundation tries to move a card to its foundation.
// Used for double-click convenience.
func (g *Game) AutoMoveToFoundation(src SourceType, srcCol int) bool {
	switch src {
	case SourceWaste:
		return g.MoveWasteToFoundation()
	case SourceTableau:
		return g.MoveTableauToFoundation(srcCol)
	}
	return false
}

// AutoMoveToTableau tries to move the top waste card to any
// valid tableau column.
func (g *Game) AutoMoveToTableau(src SourceType) bool {
	if src != SourceWaste {
		return false
	}
	for col := range 7 {
		if g.MoveWasteToTableau(col) {
			return true
		}
	}
	return false
}

// AllFaceUp reports whether every card remaining in the tableau
// and waste is face-up (auto-complete condition).
func (g *Game) AllFaceUp() bool {
	if len(g.Stock) > 0 || len(g.Waste) > 0 {
		return false
	}
	for col := range 7 {
		for _, c := range g.Tableau[col] {
			if !c.FaceUp {
				return false
			}
		}
	}
	return true
}

// AutoComplete moves all remaining tableau cards to foundations.
// Returns true if at least one card was moved.
func (g *Game) AutoComplete() bool {
	moved := false
	for {
		progress := false
		for col := range 7 {
			if g.MoveTableauToFoundation(col) {
				progress = true
				moved = true
			}
		}
		if !progress {
			break
		}
	}
	return moved
}

// TotalFoundationCards returns the total number of cards across
// all foundation piles.
func (g *Game) TotalFoundationCards() int {
	n := 0
	for i := range g.Foundation {
		n += len(g.Foundation[i])
	}
	return n
}

func (g *Game) flipTopCard(col int) {
	pile := g.Tableau[col]
	if len(pile) > 0 && !pile[len(pile)-1].FaceUp {
		pile[len(pile)-1].FaceUp = true
		g.Score += 5
	}
}

func (g *Game) checkWin() {
	if g.TotalFoundationCards() == 52 {
		g.State = StateWon
	}
}
