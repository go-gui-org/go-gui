package main

import "testing"

func TestOrbitPeriodReproducesTable(t *testing.T) {
	cases := []struct {
		name string
		a    float32
		want float32
	}{
		{"Mercury", planets[0].OrbitA, 6.0},
		{"Earth", planets[2].OrbitA, 11.4},
		{"Jupiter", planets[4].OrbitA, 34.6},
	}
	for _, tc := range cases {
		got := orbitPeriod(tc.a)
		if diff := got - tc.want; diff < -0.15 || diff > 0.15 {
			t.Fatalf("%s: orbitPeriod(%v)=%v want %v (±0.15)", tc.name, tc.a, got, tc.want)
		}
	}
}

func TestEllipsePosMatchesOrbitPos(t *testing.T) {
	for i, p := range planets {
		for _, when := range []float32{0, 1.2, 11.4, 55} {
			x1, y1 := orbitPos(&p, when)
			x2, y2 := ellipsePos(p.OrbitA, p.Ecc, p.Phase, p.PeriodS, when)
			if dx := x1 - x2; dx < -1e-4 || dx > 1e-4 {
				t.Fatalf("planet %d at t=%v: x mismatch %v vs %v", i, when, x1, x2)
			}
			if dy := y1 - y2; dy < -1e-4 || dy > 1e-4 {
				t.Fatalf("planet %d at t=%v: y mismatch %v vs %v", i, when, y1, y2)
			}
		}
	}
}

func TestBeltRocksPeriodBetweenMarsAndJupiter(t *testing.T) {
	marsP := planets[3].PeriodS
	jupP := planets[4].PeriodS
	if len(beltRocks) != beltCount {
		t.Fatalf("beltRocks len %d want %d", len(beltRocks), beltCount)
	}
	for i, r := range beltRocks {
		if r.periodS <= marsP || r.periodS >= jupP {
			t.Fatalf("rock %d period %v not strictly between Mars %v and Jupiter %v", i, r.periodS, marsP, jupP)
		}
		if r.orbitA < beltInner-1e-3 || r.orbitA > beltOuter+1e-3 {
			t.Fatalf("rock %d orbitA %v outside [%v,%v]", i, r.orbitA, beltInner, beltOuter)
		}
	}
}

func TestBeltInnerRocksLapOuter(t *testing.T) {
	// Inner rock at beltInner should have smaller period than one at beltOuter.
	inner := orbitPeriod(beltInner)
	outer := orbitPeriod(beltOuter)
	if inner >= outer {
		t.Fatalf("inner period %v should be < outer %v", inner, outer)
	}
}

func TestOrbitPeriodDegenerateInputs(t *testing.T) {
	for _, a := range []float32{0, -10, -1e6} {
		got := orbitPeriod(a)
		if got <= 0 || got != got || got > 1e9 {
			t.Fatalf("orbitPeriod(%v)=%v want finite positive", a, got)
		}
	}
}

func TestEllipsePosDegenerateInputs(t *testing.T) {
	// Zero/negative period and out-of-range ecc must not produce NaN/Inf.
	cases := []struct {
		a, ecc, phase, periodS, t float32
	}{
		{250, 0.1, 0, 0, 10},
		{250, 0.1, 0, -5, 10},
		{250, -0.5, 0, 10, 5},
		{250, 1.5, 0, 10, 5},
		{250, 0.99, 0, 10, 1e6},
	}
	for i, c := range cases {
		x, y := ellipsePos(c.a, c.ecc, c.phase, c.periodS, c.t)
		if x != x || y != y {
			t.Fatalf("case %d: ellipsePos returned NaN (%v,%v)", i, x, y)
		}
		if x > 1e9 || x < -1e9 || y > 1e9 || y < -1e9 {
			t.Fatalf("case %d: ellipsePos out of range (%v,%v)", i, x, y)
		}
	}
}
