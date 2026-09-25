// Thinking-orbs geometry modes, ported term for term from
// OrbEngine.swift in ThinkingOrbs by Haplo LLC (MIT), itself
// hand-ported from Jakub Antalik's thinking-orbs (MIT). One
// builder per mode; shared primitives live in
// thinking_orb_engine.go. See that file for attribution.
package gui

import (
	"math"
	"slices"
)

// orbOrbits draws working: particles on tilted orbits.
func orbOrbits(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	cx := size / 2
	cy := size / 2
	rr := (size / 2) * 0.82
	pt := newOrbProjector(t*0.12, 0.3, cx, cy, 1)
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))

	orbitN := orbCountOpt(o, "orbitN", 12)
	ghostN := orbCountOpt(o, "ghostN", 40)
	particles := orbCountOpt(o, "particles", 3)
	ghostR := orbOpt(o, "ghostR", 0.9)
	ghostA := orbOpt(o, "ghostA", 0.5)
	partR := orbOpt(o, "partR", 1.2)
	partRDepth := orbOpt(o, "partRDepth", 1.6)

	dots := sc.dots[:0]

	for orb := 0; orb < orbBelow(orbitN); orb++ {
		h1 := orbHash(float64(orb), 1.7)
		h2 := orbHash(float64(orb), 5.2)
		h3 := orbHash(float64(orb), 8.9)
		ro := rr * (0.45 + 0.52*h1)
		th := h1 * 2 * math.Pi
		phi := math.Acos(2*h2 - 1)
		// Orbit plane basis (u, v ⟂ normal n).
		nx := math.Sin(phi) * math.Cos(th)
		ny := math.Cos(phi)
		nz := math.Sin(phi) * math.Sin(th)
		ux := -ny
		uy := nx
		uz := 0.0
		ul := math.Max(1e-6, math.Sqrt(ux*ux+uy*uy))
		ux /= ul
		uy /= ul
		vx := ny*uz - nz*uy
		vy := nz*ux - nx*uz
		vz := nx*uy - ny*ux
		speed := (0.25 + 0.55*h3)
		if h3 <= 0.5 {
			speed = -speed
		}

		// Ghost path.
		for kk := 0; kk < orbBelow(ghostN); kk++ {
			aa := (float64(kk) / ghostN) * 2 * math.Pi
			px, py, pz := pt.project(
				(ux*math.Cos(aa)+vx*math.Sin(aa))*ro,
				(uy*math.Cos(aa)+vy*math.Sin(aa))*ro,
				(uz*math.Cos(aa)+vz*math.Sin(aa))*ro)
			depth := (pz/ro + 1) / 2
			dots = append(dots, orbDot{x: px, y: py, z: pz,
				r: ghostR * rs, white: 0.72,
				a: ghostA * (0.4 + 0.6*depth)})
		}
		// The particles doing the work.
		for mm := 0; mm < orbBelow(particles); mm++ {
			aa := t*speed + (float64(mm)/particles)*2*math.Pi + h2*6
			px, py, pz := pt.project(
				(ux*math.Cos(aa)+vx*math.Sin(aa))*ro,
				(uy*math.Cos(aa)+vy*math.Sin(aa))*ro,
				(uz*math.Cos(aa)+vz*math.Sin(aa))*ro)
			depth := (pz/ro + 1) / 2
			dots = append(dots, orbDot{x: px, y: py, z: pz,
				r:     (partR + partRDepth*depth) * rs,
				white: 0.3 - 0.22*depth, a: 1})
		}
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.3))
}

// orbGlobe draws searching: a scan meridian sweeping a dotted
// globe.
func orbGlobe(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	spin := 0.5
	cx := size / 2
	cy := size / 2
	radius := (size / 2) * 0.82
	tilt := 0.4 + 0.06*math.Sin(t*0.35)
	pt := newOrbProjector(t*spin, tilt, cx, cy, radius)
	// The scan sweeps relative to the spin; scanMul scales
	// that rate.
	scan := t * (spin + (1.7-spin)*orbOpt(o, "scanMul", 1))
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))
	dimBase := orbOpt(o, "dimBase", 1)
	rBase := orbOpt(o, "rBase", 0.6)
	rDepth := orbOpt(o, "rDepth", 1.7)
	rBoost := orbOpt(o, "rBoost", 1)
	inkFar := orbOpt(o, "inkFar", 0.62)
	inkSpan := orbOpt(o, "inkSpan", 0.54)

	dots := sc.dots[:0]
	latRings := orbCountOpt(o, "latRings", 17)
	lonDensity := orbOpt(o, "lonDensity", 44)
	for li := 0; li < orbThrough(latRings); li++ {
		lat := -math.Pi/2 + (float64(li)/latRings)*math.Pi
		cosLat := math.Cos(lat)
		sinLat := math.Sin(lat)
		lonCount := math.Max(1, orbJsRound(math.Abs(cosLat)*lonDensity))
		for lj := 0; lj < int(lonCount); lj++ {
			lon := (float64(lj) / lonCount) * 2 * math.Pi
			px, py, pz := pt.project(
				cosLat*math.Cos(lon), sinLat, cosLat*math.Sin(lon))
			depth := (pz + 1) / 2
			// The scan: a moving meridian read as a size
			// ripple, not a shine.
			dd := orbAngleDelta(lon+t*spin, scan)
			boost := math.Exp(-(dd*dd)/0.18) * math.Max(0, pz)
			dots = append(dots, orbDot{
				x: px, y: py, z: pz,
				r:     (rBase + rDepth*depth + rBoost*boost) * rs,
				white: inkFar - inkSpan*depth,
				// dimBase < 1 fades un-scanned dots so the
				// meridian reads.
				a: dimBase + (1-dimBase)*math.Min(1, boost),
			})
		}
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.3))
}

// orbMove is one scramble quarter-turn of a band.
type orbMove struct {
	axis   int
	lo, hi float64
	ang    float64
}

// orbSolveCycle replays eased moves then reverses them (a
// palindrome) so everything clicks back to solved, rests,
// repeats. Returns per-move amounts and the active move.
func orbSolveCycle(time float64, count int, slotDur, rest float64) ([]float64, int) {
	amount := make([]float64, count)
	return amount, orbSolveCycleInto(amount, time, count, slotDur, rest)
}

// orbSolveCycleInto is orbSolveCycle writing the per-move amounts
// into amount (length count) and returning the active move.
func orbSolveCycleInto(amount []float64, time float64, count int,
	slotDur, rest float64) int {
	clear(amount)
	cyc := 2*float64(count)*slotDur + rest
	// Wrapped into [0, cyc) so a negative time can never index
	// before the start.
	tc := math.Mod(time, cyc)
	if tc < 0 {
		tc += cyc
	}
	active := -1
	if tc < 2*float64(count)*slotDur {
		slot := int(math.Floor(tc / slotDur))
		pp := (tc - float64(slot)*slotDur) / slotDur
		cl := math.Min(1, pp/0.7)
		ep := 1 - (1-cl)*(1-cl)*(1-cl) // machine ease-out
		if slot < count {
			for i := range slot {
				amount[i] = 1
			}
			amount[slot] = ep
			active = slot
		} else {
			u := 2*count - 1 - slot
			for i := range u {
				amount[i] = 1
			}
			amount[u] = 1 - ep
			active = u
		}
	}
	return active
}

func orbMakeMoves(count int) []orbMove {
	moves := make([]orbMove, count)
	for i := range moves {
		fi := float64(i)
		axis := math.Min(2, math.Floor(orbHash(fi, 2.3)*3))
		band := math.Min(3, math.Floor(orbHash(fi, 5.9)*4))
		lo := -1.0 + 0.5*band
		dir := 1.0
		if orbHash(fi, 7.7) >= 0.5 {
			dir = -1
		}
		moves[i] = orbMove{axis: int(axis), lo: lo, hi: lo + 0.5,
			ang: (dir * math.Pi) / 2}
	}
	return moves
}

func orbApplyMoves(px, py, pz float64, moves []orbMove, amount []float64,
	active int) (float64, float64, float64, bool) {
	x, y, z := px, py, pz
	inActive := false
	for i := range moves {
		if amount[i] <= 0 {
			continue
		}
		mv := moves[i]
		var coord float64
		switch mv.axis {
		case 0:
			coord = x
		case 1:
			coord = y
		default:
			coord = z
		}
		if coord < mv.lo || coord >= mv.hi {
			continue
		}
		if i == active {
			inActive = true
		}
		aa := mv.ang * amount[i]
		ca := math.Cos(aa)
		sa := math.Sin(aa)
		switch mv.axis {
		case 0:
			y2 := y*ca - z*sa
			z = y*sa + z*ca
			y = y2
		case 1:
			x2 := x*ca + z*sa
			z = -x*sa + z*ca
			x = x2
		default:
			x2 := x*ca - y*sa
			y = x*sa + y*ca
			x = x2
		}
	}
	return x, y, z, inActive
}

// orbRubik draws solving: rapid eased moves scramble, then
// replay in reverse.
func orbRubik(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	cx := size / 2
	cy := size / 2
	rr := (size / 2) * 0.82
	pt := newOrbProjector(t*0.55, 0.35+0.1*math.Sin(t*0.9), cx, cy, rr)
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))
	moveCount := int(orbCountOpt(o, "moveCount", 14))
	// The moves depend only on the count, so they are built once
	// per scratch and reused while the count holds.
	if len(sc.moves) != moveCount {
		sc.moves = orbMakeMoves(moveCount)
	}
	moves := sc.moves
	sc.amount = slices.Grow(sc.amount[:0], moveCount)[:moveCount]
	amount := sc.amount
	active := orbSolveCycleInto(amount, t, moveCount, 0.42, 1.2)
	rBase := orbOpt(o, "rBase", 0.6)
	rDepth := orbOpt(o, "rDepth", 1.7)
	rActive := orbOpt(o, "rActive", 0.3)
	inkFar := orbOpt(o, "inkFar", 0.62)
	inkSpan := orbOpt(o, "inkSpan", 0.54)

	dots := sc.dots[:0]
	latRings := orbCountOpt(o, "latRings", 15)
	lonDensity := orbOpt(o, "lonDensity", 40)
	for li := 0; li < orbThrough(latRings); li++ {
		lat := -math.Pi/2 + (float64(li)/latRings)*math.Pi
		cosLat := math.Cos(lat)
		sinLat := math.Sin(lat)
		lonCount := math.Max(1, orbJsRound(math.Abs(cosLat)*lonDensity))
		for lj := 0; lj < int(lonCount); lj++ {
			lon := (float64(lj) / lonCount) * 2 * math.Pi
			mx, my, mz, inActive := orbApplyMoves(
				cosLat*math.Cos(lon), sinLat, cosLat*math.Sin(lon),
				moves, amount, active)
			px, py, pz := pt.project(mx, my, mz)
			depth := (pz + 1) / 2
			extra := 0.0
			ink := 0.0
			if inActive {
				// The band being turned inks a touch
				// darker: the "hand".
				extra = rActive
				ink = 0.14
			}
			dots = append(dots, orbDot{
				x: px, y: py, z: pz,
				r:     (rBase + rDepth*depth + extra) * rs,
				white: inkFar - inkSpan*depth - ink, a: 1,
			})
		}
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.3))
}

// orbWave draws listening: a waveform rolling through the
// latitude rings.
func orbWave(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	cx := size / 2
	cy := size / 2
	// 0.76 base × 1.15: the undulation pulls the sphere inward,
	// so it is scaled up to read the same size as the other
	// lattice modes.
	rr := (size / 2) * 0.874
	pt := newOrbProjector(t*0.18, 0.38, cx, cy, 1)
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))
	rBase := orbOpt(o, "rBase", 0.6)
	rDepth := orbOpt(o, "rDepth", 1.7)

	dots := sc.dots[:0]
	rings := orbCountOpt(o, "rings", 15)
	lonDensity := orbOpt(o, "lonDensity", 40)
	for ri := 0; ri < orbThrough(rings); ri++ {
		fri := float64(ri)
		lat := -math.Pi/2 + (fri/rings)*math.Pi
		cosLat := math.Cos(lat)
		sinLat := math.Sin(lat)
		// Two waves, different tempi: organic, never quite
		// repeating.
		ww := 0.62*math.Sin(t*2.1-fri*0.52) +
			0.38*math.Sin(t*1.27+fri*0.83)
		rwave := rr * (0.88 + 0.105*ww)
		lonCount := math.Max(1, orbJsRound(math.Abs(cosLat)*lonDensity))
		for lj := 0; lj < int(lonCount); lj++ {
			lon := (float64(lj) / lonCount) * 2 * math.Pi
			px, py, pz := pt.project(cosLat*math.Cos(lon)*rwave,
				sinLat*rwave, cosLat*math.Sin(lon)*rwave)
			depth := (pz/rr + 1) / 2
			crest := math.Max(0, ww)
			dots = append(dots, orbDot{
				x: px, y: py, z: pz,
				r:     (rBase + rDepth*depth) * (1 + 0.4*crest) * rs,
				white: 0.66 - 0.56*depth - 0.1*crest, a: 1,
			})
		}
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.3))
}

// orbWeb draws connecting: a constellation wiring itself,
// packets running the edges.
func orbWeb(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	cx := size / 2
	cy := size / 2
	rr := (size / 2) * 0.8 * orbOpt(o, "spread", 1)
	// The projector carries the radius as its scale, so node
	// vectors stay unit-length and the distances below are in
	// unit-sphere space.
	pt := newOrbProjector(t*0.12, 0.32, cx, cy, rr)
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))

	nodeN := orbCountOpt(o, "nodeN", 30)
	thr := orbOpt(o, "thr", 0.72)
	nodeR := orbOpt(o, "nodeR", 1.4)
	nodeRDepth := orbOpt(o, "nodeRDepth", 1.8)
	lineW := orbOpt(o, "lineW", 0.8)

	// Nodes: fib lattice + slow noise wander, renormalised to
	// the surface.
	nodeCount := orbBelow(nodeN)
	nodes := sc.nodes[:0]
	for i := range nodeCount {
		fi := float64(i)
		dx, dy, dz := orbFibDir(i, nodeN)
		nx := dx + 0.3*(orbVnoise(fi*0.31+9, t*0.24)-0.5)*2
		ny := dy + 0.3*(orbVnoise(fi*0.53+27, t*0.21)-0.5)*2
		nz := dz + 0.3*(orbVnoise(fi*0.77+55, t*0.27)-0.5)*2
		ll := math.Sqrt(nx*nx + ny*ny + nz*nz)
		nodes = append(nodes, [3]float64{nx / ll, ny / ll, nz / ll})
	}
	sc.nodes = nodes

	lines := sc.lines[:0]
	dots := sc.dots[:0]

	// Edges between close neighbours, alpha by proximity and
	// depth.
	for ii := range nodeCount {
		for jj := ii + 1; jj < nodeCount; jj++ {
			dx := nodes[ii][0] - nodes[jj][0]
			dy := nodes[ii][1] - nodes[jj][1]
			dz := nodes[ii][2] - nodes[jj][2]
			dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if dist >= thr {
				continue
			}
			x1, y1, z1 := pt.project(nodes[ii][0], nodes[ii][1], nodes[ii][2])
			x2, y2, z2 := pt.project(nodes[jj][0], nodes[jj][1], nodes[jj][2])
			depth := ((z1+z2)/2 + 1) / 2
			lines = append(lines, orbLine{
				x1: x1, y1: y1, x2: x2, y2: y2,
				white: 0.42,
				a:     (1 - dist/thr) * (0.3 + 0.55*depth),
				w:     math.Max(0.6, lineW*rs),
			})
		}
	}

	for i := range nodes {
		px, py, pz := pt.project(nodes[i][0], nodes[i][1], nodes[i][2])
		depth := (pz + 1) / 2
		pulse := 1 + 0.25*math.Sin(t*1.4+float64(i)*2.7)
		dots = append(dots, orbDot{x: px, y: py, z: pz,
			r:     (nodeR + nodeRDepth*depth) * pulse * rs,
			white: 0.55 - 0.45*depth, a: 1})
	}

	// Signals: bright packets running between re-picked node
	// pairs.
	signals := orbCountOpt(o, "signals", 5)
	for s := 0; s < orbBelow(signals); s++ {
		fs := float64(s)
		seg := math.Floor(t*0.55 + fs*7.31)
		aIdx := int(math.Floor(orbHash(seg, fs*3.1+1.7) * nodeN))
		bIdx := int(math.Floor(orbHash(seg, fs*5.7+4.2) * nodeN))
		if aIdx == bIdx {
			continue
		}
		ff := orbFrac(t*0.55 + fs*7.31)
		sx := orbLerp(nodes[aIdx][0], nodes[bIdx][0], ff)
		sy := orbLerp(nodes[aIdx][1], nodes[bIdx][1], ff)
		sz := orbLerp(nodes[aIdx][2], nodes[bIdx][2], ff)
		ll := math.Max(1e-6, math.Sqrt(sx*sx+sy*sy+sz*sz))
		px, py, pz := pt.project(sx/ll, sy/ll, sz/ll)
		depth := (pz + 1) / 2
		dots = append(dots, orbDot{x: px, y: py, z: pz,
			r:     (nodeR*1.5 + nodeRDepth*depth) * rs,
			white: 0.05, a: 0.5 + 0.5*depth})
	}

	return orbFinalize(sc, dots, lines, orbOpt(o, "rMin", 0.3))
}

// orbBraid draws weaving: three strands plaiting around the
// sphere.
func orbBraid(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	cx := size / 2
	cy := size / 2
	rr := (size / 2) * 0.76
	pt := newOrbProjector(t*0.4, 0.3, cx, cy, 1)
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))
	rBase := orbOpt(o, "rBase", 1.2)
	rDepth := orbOpt(o, "rDepth", 1.8)

	dots := sc.dots[:0]
	ghostN := orbCountOpt(o, "ghostN", 150)
	for i := 0; i < orbBelow(ghostN); i++ {
		dx, dy, dz := orbFibDir(i, ghostN)
		px, py, pz := pt.project(dx*rr, dy*rr, dz*rr)
		depth := (pz/rr + 1) / 2
		dots = append(dots, orbDot{x: px, y: py, z: pz,
			r: 0.8 * rs, white: 0.78, a: 0.1 + 0.22*depth})
	}

	strandN := orbCountOpt(o, "strandN", 52)
	turns := orbOpt(o, "turns", 3)
	for s := range 3 {
		phase := (float64(s) / 3) * 2 * math.Pi
		for i := 0; i < orbBelow(strandN); i++ {
			// u walks pole to pole; the frac drift slides
			// the strand along.
			u := (orbFrac(float64(i)/strandN+t*0.045)*2 - 1) * 0.96
			surf := math.Sqrt(math.Max(0, 1-u*u))
			endFade := math.Min(1, (1-math.Abs(u))/0.1)
			aa := u*math.Pi*turns + phase
			// Radial breathing: strands trade places, the
			// over/under of a plait.
			weave := 1 + 0.075*math.Sin(u*math.Pi*turns*2+phase*2+t*0.8)
			rwave := surf * rr * weave
			px, py, pz := pt.project(math.Cos(aa)*rwave,
				u*rr*weave, math.Sin(aa)*rwave)
			depth := (pz/rr + 1) / 2
			dots = append(dots, orbDot{
				x: px, y: py, z: pz,
				r:     (rBase + rDepth*depth) * rs,
				white: 0.55 - 0.45*depth,
				a:     endFade * (0.45 + 0.55*depth),
			})
		}
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.3))
}

// orbRibbon draws composing (and breathing via faceOn): an
// undulating band, frozen in place while waves travel along it.
func orbRibbon(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	cx := size / 2
	cy := size / 2
	rr := (size / 2) * 0.78
	// Spin scales the 3D tumble; 0 freezes the band's
	// orientation, leaving only the traveling undulation.
	spin := orbOpt(o, "spin", 1)
	camTilt := 0.3
	pt := newOrbProjector(t*0.1*spin, camTilt, cx, cy, 1)
	rs := orbRadiusScale(size, orbOpt(o, "rsPow", 0.6))
	faceOn := orbOpt(o, "faceOn", 0) != 0
	wobMul := orbOpt(o, "wobMul", 1)
	rBase := orbOpt(o, "rBase", 1.1)
	rDepth := orbOpt(o, "rDepth", 1.7)

	dots := sc.dots[:0]
	ghostN := orbCountOpt(o, "ghostN", 150)
	for i := 0; i < orbBelow(ghostN); i++ {
		dx, dy, dz := orbFibDir(i, ghostN)
		px, py, pz := pt.project(dx*rr, dy*rr, dz*rr)
		depth := (pz/rr + 1) / 2
		dots = append(dots, orbDot{x: px, y: py, z: pz,
			r: 0.8 * rs, white: 0.78, a: 0.1 + 0.22*depth})
	}

	// The band plane, precessing (frozen when spin = 0).
	// Face-on sets ta = -camTilt so the band reads as a true
	// circle, not an ellipse.
	ya := t * 0.24 * spin
	ta := 0.55 + 0.3*math.Sin(t*0.18)*spin
	if faceOn {
		ta = -camTilt
	}
	ux := math.Cos(ya)
	uy := 0.0
	uz := math.Sin(ya)
	vx := -uz * math.Sin(ta)
	vy := math.Cos(ta)
	vz := ux * math.Sin(ta)
	// Plane normal n = u × v.
	nx := uy*vz - uz*vy
	ny := uz*vx - ux*vz
	nz := ux*vy - uy*vx

	// Radial lobes swell past R, so face-on pulls the base
	// radius in by most of the wobble amplitude to keep the
	// silhouette in frame.
	wobAmp := 0.23 * wobMul
	baseR := rr
	if faceOn {
		baseR = rr / (1 + 0.85*wobAmp)
	}

	baseLanes := orbOpt(o, "lanes", 5)
	segs := orbCountOpt(o, "segs", 88)
	lanes := math.Max(1, orbJsRound(baseLanes*orbOpt(o, "bandMul", 1)))
	mid := (lanes - 1) / 2
	for w := 0; w < lanesInt(lanes); w++ {
		fw := float64(w)
		laneOff := (fw - mid) * 0.075
		edge := math.Abs(fw-mid) / math.Max(1, mid)
		for kk := 0; kk < orbBelow(segs); kk++ {
			aa := (float64(kk) / segs) * 2 * math.Pi
			// Two traveling waves along the band; wobMul
			// scales the deformation, 0 is a clean band.
			wob := (0.16*math.Sin(aa*3-t*1.7+fw*0.22) +
				0.07*math.Sin(aa*5+t*1.1)) * wobMul
			// Ribbon wobbles out of plane (renormalised
			// back onto the sphere); face-on modulates
			// the in-plane radius instead.
			radial := 1.0
			off := laneOff + wob
			if faceOn {
				radial = 1 + wob
				off = laneOff
			}
			bx := ux*math.Cos(aa) + vx*math.Sin(aa) + nx*off
			by := uy*math.Cos(aa) + vy*math.Sin(aa) + ny*off
			bz := uz*math.Cos(aa) + vz*math.Sin(aa) + nz*off
			ll := math.Sqrt(bx*bx + by*by + bz*bz)
			rwave := baseR * radial
			px, py, pz := pt.project(
				(bx/ll)*rwave, (by/ll)*rwave, (bz/ll)*rwave)
			depth := (pz/rr + 1) / 2
			dots = append(dots, orbDot{
				x: px, y: py, z: pz,
				r:     (rBase + rDepth*depth) * (1 - 0.25*edge) * rs,
				white: 0.52 - 0.44*depth + 0.18*edge,
				a:     0.4 + 0.6*depth,
			})
		}
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.3))
}

// lanesInt converts the lanes float (already rounded) to an int
// loop bound.
func lanesInt(lanes float64) int {
	if lanes < 1 {
		return 1
	}
	return int(lanes)
}

// orbPolyPath is a closed path parameterised by arc length.
type orbPolyPath struct {
	verts   [][2]float64
	lengths []float64
	total   float64
}

func newOrbPolyPath(verts [][2]float64) orbPolyPath {
	pp := orbPolyPath{verts: verts, lengths: make([]float64, len(verts))}
	for i := range verts {
		aa := verts[i]
		bb := verts[(i+1)%len(verts)]
		ll := math.Hypot(bb[0]-aa[0], bb[1]-aa[1])
		pp.lengths[i] = ll
		pp.total += ll
	}
	return pp
}

func (p orbPolyPath) at(f float64) (float64, float64) {
	target := f * p.total
	idx := 0
	for target > p.lengths[idx] && idx < len(p.verts)-1 {
		target -= p.lengths[idx]
		idx++
	}
	aa := p.verts[idx]
	bb := p.verts[(idx+1)%len(p.verts)]
	ff := 0.0
	if p.lengths[idx] != 0 {
		ff = math.Min(1, target/p.lengths[idx])
	}
	return aa[0] + (bb[0]-aa[0])*ff, aa[1] + (bb[1]-aa[1])*ff
}

// orbTrianglePath starts at top-centre like the other shapes;
// the square walks 5 vertices for the same reason.
var orbTrianglePath = newOrbPolyPath([][2]float64{
	{0.0, -0.26}, {0.24, 0.16}, {-0.24, 0.16},
})

var orbSquarePath = newOrbPolyPath([][2]float64{
	{0, -0.2}, {0.2, -0.2}, {0.2, 0.2}, {-0.2, 0.2}, {-0.2, -0.2},
})

func orbShapePoint(shape int, f float64) (float64, float64) {
	switch shape {
	case 0:
		aa := -math.Pi/2 + f*2*math.Pi
		return math.Cos(aa) * 0.24, math.Sin(aa) * 0.24
	case 1:
		return orbTrianglePath.at(f)
	default:
		return orbSquarePath.at(f)
	}
}

const (
	orbMorphHold = 1.4
	orbMorphTime = 0.9
)

// orbMorph draws shaping: a dotted outline morphing circle to
// triangle to square. Each shape is a closed path parameterised
// by arc length (top-centre start, clockwise). Every frame
// blends the two neighbouring paths, then lays the dots evenly
// along the blended outline.
func orbMorph(sc *orbScratch, size, t float64, o orbOpts) orbFrameResult {
	shapeCount := 3
	segDur := orbMorphHold + orbMorphTime
	tc := math.Mod(t, segDur*float64(shapeCount))
	if tc < 0 {
		tc += segDur * float64(shapeCount)
	}
	kk := int(math.Floor(tc / segDur))
	local := tc - float64(kk)*segDur
	mm := 0.0
	if local > orbMorphHold {
		xx := (local - orbMorphHold) / orbMorphTime
		mm = xx * xx * (3 - 2*xx)
	}
	sprd := orbOpt(o, "spread", 1)

	// Blend the two shape paths at m, then measure the blended
	// outline.
	const pathSamples = 160
	// Fixed-size arrays stay on the stack: no heap per frame.
	var pts [pathSamples][2]float64
	for i := range pathSamples {
		ff := float64(i) / float64(pathSamples)
		ax, ay := orbShapePoint(kk, ff)
		bx, by := orbShapePoint((kk+1)%shapeCount, ff)
		pts[i] = [2]float64{
			(ax + (bx-ax)*mm) * sprd, (ay + (by-ay)*mm) * sprd}
	}
	var segLens [pathSamples]float64
	total := 0.0
	for i := range pts {
		aa := pts[i]
		bb := pts[(i+1)%pathSamples]
		ll := math.Hypot(bb[0]-aa[0], bb[1]-aa[1])
		segLens[i] = ll
		total += ll
	}

	// Dot radius depends only on rDot; the count sets the gaps.
	// Formed shapes breathe a little (a uniform pulse).
	dotN := math.Max(6, orbJsRound(34*orbOpt(o, "iconD", 1)))
	re := orbOpt(o, "rDot", 0.021) * 1.35 * sprd
	pulse := 1 + 0.02*math.Sin(local*3.1)

	dots := sc.dots[:0]
	c2 := size / 2
	seg := 0
	acc := 0.0
	for k2 := 0; k2 < int(dotN); k2++ {
		target := (float64(k2) / dotN) * total
		for acc+segLens[seg] < target && seg < pathSamples-1 {
			acc += segLens[seg]
			seg++
		}
		aa := pts[seg]
		bb := pts[(seg+1)%pathSamples]
		ff := 0.0
		if segLens[seg] != 0 {
			ff = math.Min(1, (target-acc)/segLens[seg])
		}
		xx := (aa[0] + (bb[0]-aa[0])*ff) * pulse
		yy := (aa[1] + (bb[1]-aa[1])*ff) * pulse
		dots = append(dots, orbDot{x: c2 + xx*size, y: c2 + yy*size,
			z: 0, r: math.Max(0.35, re*size), white: 0.1, a: 1})
	}
	return orbFinalize(sc, dots, nil, orbOpt(o, "rMin", 0.25))
}
