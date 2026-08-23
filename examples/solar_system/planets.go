package main

import "github.com/go-gui-org/go-gui/gui"

// Planet is one body's orbit, appearance, and fact-sheet copy. Every
// field is static: the simulation derives position from Time alone, so
// this table is read-only after init.
type Planet struct {
	Name string

	// OrbitA is the semi-major axis in world units and PeriodS the
	// seconds for one full orbit.
	//
	// Both are *compressed* rather than to scale. Real orbital periods
	// span 88 days (Mercury) to 60190 days (Neptune) — a 684x range no
	// one can watch, since Neptune would not visibly move before
	// Mercury lapped it 684 times. These are realDays^0.45, scaled so
	// Mercury takes ~6s: the ordering is exact and every gap is
	// visible. Orbit radii get the same treatment at realAU^0.38 x 195
	// — a milder exponent than the periods use, because a stronger one
	// packs the inner four inside the sun's halo.
	OrbitA  float32
	PeriodS float32

	// Ecc is orbital eccentricity — the real value, since these are
	// already in a range the eye can see without exaggeration.
	Ecc float32

	// Phase is the starting angle in radians, chosen so the planets do
	// not begin in a line.
	Phase float32

	// Radius is the drawn body radius in world units. Compressed like
	// the orbits: Jupiter is really 28x Mercury, which would make
	// Mercury sub-pixel at any zoom that fits Jupiter.
	Radius float32

	Color gui.Color

	// Fact-sheet copy, shown in the info panel.
	Diameter    string
	Mass        string
	Distance    string
	DayLength   string
	Gravity     string
	Temperature string
	FunFact     string
}

// planets is ordered sun-outward; index is the identity used by
// selection, nav dots, and arrow-key stepping.
var planets = [...]Planet{
	{
		Name: "Mercury", OrbitA: 136, PeriodS: 6.0, Ecc: 0.206, Phase: 0.4,
		Radius: 7, Color: gui.RGB(168, 158, 148),
		Diameter: "4,879 km", Mass: "0.055 Earths", Distance: "0.39 AU",
		DayLength: "1,408 h", Gravity: "3.7 m/s²", Temperature: "-173 to 427 °C",
		FunFact: "A day on Mercury lasts longer than its year — it spins " +
			"three times for every two trips around the Sun.",
	},
	{
		Name: "Venus", OrbitA: 172, PeriodS: 9.2, Ecc: 0.007, Phase: 2.1,
		Radius: 11, Color: gui.RGB(222, 184, 135),
		Diameter: "12,104 km", Mass: "0.815 Earths", Distance: "0.72 AU",
		DayLength: "5,832 h", Gravity: "8.9 m/s²", Temperature: "464 °C",
		FunFact: "Venus turns backwards, so the Sun there rises in the " +
			"west and sets in the east.",
	},
	{
		Name: "Earth", OrbitA: 195, PeriodS: 11.4, Ecc: 0.017, Phase: 4.3,
		Radius: 12, Color: gui.RGB(76, 140, 214),
		Diameter: "12,742 km", Mass: "1 Earth", Distance: "1.00 AU",
		DayLength: "24 h", Gravity: "9.8 m/s²", Temperature: "-88 to 58 °C",
		FunFact: "The only world known to have liquid water on its " +
			"surface — and the only one known to have anyone looking at it.",
	},
	{
		Name: "Mars", OrbitA: 229, PeriodS: 15.1, Ecc: 0.093, Phase: 0.9,
		Radius: 9, Color: gui.RGB(193, 91, 58),
		Diameter: "6,779 km", Mass: "0.107 Earths", Distance: "1.52 AU",
		DayLength: "24.7 h", Gravity: "3.7 m/s²", Temperature: "-153 to 20 °C",
		FunFact: "Olympus Mons is the tallest volcano in the solar system " +
			"— nearly three times the height of Everest.",
	},
	{
		Name: "Jupiter", OrbitA: 365, PeriodS: 34.6, Ecc: 0.049, Phase: 3.4,
		Radius: 32, Color: gui.RGB(201, 156, 111),
		Diameter: "139,820 km", Mass: "318 Earths", Distance: "5.20 AU",
		DayLength: "9.9 h", Gravity: "24.8 m/s²", Temperature: "-108 °C",
		FunFact: "The Great Red Spot is a storm wider than Earth that has " +
			"been blowing for at least 150 years.",
	},
	{
		Name: "Saturn", OrbitA: 459, PeriodS: 52.2, Ecc: 0.057, Phase: 5.6,
		Radius: 28, Color: gui.RGB(224, 202, 148),
		Diameter: "116,460 km", Mass: "95 Earths", Distance: "9.54 AU",
		DayLength: "10.7 h", Gravity: "10.4 m/s²", Temperature: "-138 °C",
		FunFact: "Saturn is less dense than water — given a bathtub big " +
			"enough, it would float.",
	},
	{
		Name: "Uranus", OrbitA: 599, PeriodS: 83.6, Ecc: 0.046, Phase: 1.7,
		Radius: 20, Color: gui.RGB(140, 209, 216),
		Diameter: "50,724 km", Mass: "14.5 Earths", Distance: "19.2 AU",
		DayLength: "17.2 h", Gravity: "8.7 m/s²", Temperature: "-195 °C",
		FunFact: "Uranus orbits on its side, so each pole spends 42 years " +
			"in continuous sunlight and 42 in darkness.",
	},
	{
		Name: "Neptune", OrbitA: 711, PeriodS: 113.2, Ecc: 0.011, Phase: 3.9,
		Radius: 19, Color: gui.RGB(62, 102, 209),
		Diameter: "49,244 km", Mass: "17.1 Earths", Distance: "30.1 AU",
		DayLength: "16.1 h", Gravity: "11.2 m/s²", Temperature: "-201 °C",
		FunFact: "Neptune's winds reach 2,100 km/h — the fastest measured " +
			"anywhere in the solar system.",
	},
}

// saturnIndex special-cases ring drawing without matching on Name.
const saturnIndex = 5

// selSun is the selection value for the star at the center. Selection
// is otherwise a planet index, with -1 meaning the full-system view, so
// the sun needs a value outside both ranges rather than a slot in the
// planets table: it does not orbit, is not hit-tested against a cached
// screen position that a tick recomputes from orbitPos, and is not
// drawn by drawPlanet.
const selSun = -2

// sun is the fact-sheet entry for the center of the system. It reuses
// Planet for its copy fields only — the orbit terms stay zero, since
// the sun sits at the world origin — and carries Radius so the camera
// can frame it with the same code that frames a planet.
var sun = Planet{
	// Color is the halo's gold rather than the near-white disc: it is
	// read only by the nav dot, and a cream dot at 45% opacity over
	// this background reads as grey next to Mercury's.
	Name: "Sun", Radius: sunRadius, Color: colorSunGlow,
	Diameter: "1,391,400 km", Mass: "333,000 Earths", Distance: "0 AU",
	DayLength: "609 h", Gravity: "274 m/s²", Temperature: "5,500 °C",
	FunFact: "The Sun holds 99.86% of the mass of the solar system. " +
		"Everything else — all eight planets included — is what was " +
		"left over.",
}

// bodyAt returns the body a selection or hover index names, or nil for
// "nothing". It is the one place the selSun encoding is decoded.
func bodyAt(i int) *Planet {
	switch {
	case i == selSun:
		return &sun
	case i >= 0 && i < len(planets):
		return &planets[i]
	default:
		return nil
	}
}
