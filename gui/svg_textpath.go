package gui

import (
	"math"
	"strconv"
	"unicode"
)

type cachedDefsPathData struct {
	polyline []float32
	table    []float32
	totalLen float32
}

// flattenDefsPath parses an SVG path d attribute and flattens it
// to a polyline with coordinates scaled by scale. Supports M, L,
// C, Q, A commands (absolute and relative).
//
//nolint:gocyclo // SVG path command switch
func flattenDefsPath(d string, scale float32) []float32 {
	tokens := tokenizeSvgPath(d)
	if len(tokens) == 0 {
		return nil
	}

	var out []float32
	var cx, cy float32 // current position
	var startX, startY float32
	var lastCtrlX, lastCtrlY float32
	var lastCmd string
	i := 0

	for i < len(tokens) {
		cmd := tokens[i]
		i++

		switch cmd {
		case "M", "m":
			rel := cmd == "m"
			for i+1 < len(tokens) && !isSvgCommand(tokens[i]) {
				x := parseFloat(tokens[i])
				y := parseFloat(tokens[i+1])
				i += 2
				if rel {
					x += cx
					y += cy
				}
				cx, cy = x, y
				startX, startY = cx, cy
				out = append(out, cx*scale, cy*scale)
			}

		case "L", "l":
			rel := cmd == "l"
			for i+1 < len(tokens) && !isSvgCommand(tokens[i]) {
				x := parseFloat(tokens[i])
				y := parseFloat(tokens[i+1])
				i += 2
				if rel {
					x += cx
					y += cy
				}
				cx, cy = x, y
				out = append(out, cx*scale, cy*scale)
			}

		case "H", "h":
			rel := cmd == "h"
			for i < len(tokens) && !isSvgCommand(tokens[i]) {
				x := parseFloat(tokens[i])
				i++
				if rel {
					x += cx
				}
				cx = x
				out = append(out, cx*scale, cy*scale)
			}

		case "V", "v":
			rel := cmd == "v"
			for i < len(tokens) && !isSvgCommand(tokens[i]) {
				y := parseFloat(tokens[i])
				i++
				if rel {
					y += cy
				}
				cy = y
				out = append(out, cx*scale, cy*scale)
			}

		case "C", "c":
			rel := cmd == "c"
			for i+5 < len(tokens) && !isSvgCommand(tokens[i]) {
				x1 := parseFloat(tokens[i])
				y1 := parseFloat(tokens[i+1])
				x2 := parseFloat(tokens[i+2])
				y2 := parseFloat(tokens[i+3])
				x := parseFloat(tokens[i+4])
				y := parseFloat(tokens[i+5])
				i += 6
				if rel {
					x1 += cx
					y1 += cy
					x2 += cx
					y2 += cy
					x += cx
					y += cy
				}
				flattenCubic(&out, cx, cy, x1, y1, x2, y2, x, y, scale)
				lastCtrlX, lastCtrlY = x2, y2
				cx, cy = x, y
			}

		case "S", "s":
			rel := cmd == "s"
			for i+3 < len(tokens) && !isSvgCommand(tokens[i]) {
				// Reflect control point from previous C/c/S/s.
				var c1x, c1y float32
				if lastCmd == "C" || lastCmd == "c" ||
					lastCmd == "S" || lastCmd == "s" {
					c1x = cx*2 - lastCtrlX
					c1y = cy*2 - lastCtrlY
				} else {
					c1x = cx
					c1y = cy
				}
				c2x := parseFloat(tokens[i])
				c2y := parseFloat(tokens[i+1])
				x := parseFloat(tokens[i+2])
				y := parseFloat(tokens[i+3])
				i += 4
				if rel {
					c2x += cx
					c2y += cy
					x += cx
					y += cy
				}
				flattenCubic(&out, cx, cy, c1x, c1y,
					c2x, c2y, x, y, scale)
				lastCtrlX, lastCtrlY = c2x, c2y
				cx, cy = x, y
			}

		case "Q", "q":
			rel := cmd == "q"
			for i+3 < len(tokens) && !isSvgCommand(tokens[i]) {
				x1 := parseFloat(tokens[i])
				y1 := parseFloat(tokens[i+1])
				x := parseFloat(tokens[i+2])
				y := parseFloat(tokens[i+3])
				i += 4
				if rel {
					x1 += cx
					y1 += cy
					x += cx
					y += cy
				}
				flattenQuadratic(&out, cx, cy, x1, y1, x, y, scale)
				lastCtrlX, lastCtrlY = x1, y1
				cx, cy = x, y
			}

		case "T", "t":
			rel := cmd == "t"
			for i+1 < len(tokens) && !isSvgCommand(tokens[i]) {
				// Reflect control point from previous Q/q/T/t.
				var c1x, c1y float32
				if lastCmd == "Q" || lastCmd == "q" ||
					lastCmd == "T" || lastCmd == "t" {
					c1x = cx*2 - lastCtrlX
					c1y = cy*2 - lastCtrlY
				} else {
					c1x = cx
					c1y = cy
				}
				x := parseFloat(tokens[i])
				y := parseFloat(tokens[i+1])
				i += 2
				if rel {
					x += cx
					y += cy
				}
				flattenQuadratic(&out, cx, cy, c1x, c1y,
					x, y, scale)
				lastCtrlX, lastCtrlY = c1x, c1y
				cx, cy = x, y
			}

		case "A", "a":
			rel := cmd == "a"
			for i+6 < len(tokens) && !isSvgCommand(tokens[i]) {
				rx := parseFloat(tokens[i])
				ry := parseFloat(tokens[i+1])
				rot := parseFloat(tokens[i+2])
				largeArc := parseFloat(tokens[i+3])
				sweep := parseFloat(tokens[i+4])
				x := parseFloat(tokens[i+5])
				y := parseFloat(tokens[i+6])
				i += 7
				if rel {
					x += cx
					y += cy
				}
				flattenArc(&out, cx, cy, rx, ry, rot,
					largeArc != 0, sweep != 0, x, y, scale)
				cx, cy = x, y
			}

		case "Z", "z":
			cx, cy = startX, startY
			out = append(out, cx*scale, cy*scale)

		default:
			// Skip unknown commands.
		}
		lastCmd = cmd
	}
	return out
}

// tokenizeSvgPath splits an SVG path d attribute into command
// letters and numeric tokens.
func tokenizeSvgPath(d string) []string {
	tokens := make([]string, 0, len(d)/4+1)
	for i := 0; i < len(d); {
		c := d[i]
		if unicode.IsSpace(rune(c)) || c == ',' {
			i++
			continue
		}
		if isSvgCommandRune(rune(c)) {
			tokens = append(tokens, d[i:i+1])
			i++
			continue
		}
		start := i
		hasDot := false
		for i < len(d) {
			ch := d[i]
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == ',' {
				break
			}
			if isSvgCommandRune(rune(ch)) {
				break
			}
			if ch == '.' {
				if hasDot {
					break
				}
				hasDot = true
				i++
				continue
			}
			if (ch == '-' || ch == '+') && i > start {
				prev := d[i-1]
				if prev != 'e' && prev != 'E' {
					break
				}
			}
			if (ch >= '0' && ch <= '9') ||
				ch == '-' || ch == '+' ||
				ch == 'e' || ch == 'E' {
				i++
				continue
			}
			break
		}
		if i > start {
			tokens = append(tokens, d[start:i])
			continue
		}
		i++
	}
	return tokens
}

func isSvgCommand(s string) bool {
	if len(s) != 1 {
		return false
	}
	return isSvgCommandRune(rune(s[0]))
}

func isSvgCommandRune(ch rune) bool {
	switch ch {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v',
		'C', 'c', 'Q', 'q', 'A', 'a', 'Z', 'z',
		'S', 's', 'T', 't':
		return true
	}
	return false
}

func parseFloat(s string) float32 {
	v, _ := strconv.ParseFloat(s, 32)
	return float32(v)
}

// flattenCubic approximates a cubic Bezier with line segments.
// Step count adapts to chord length for accuracy on large curves
// without over-tessellating small ones.
func flattenCubic(out *[]float32,
	x0, y0, x1, y1, x2, y2, x3, y3, scale float32) {
	steps := adaptiveSteps(x0, y0, x3, y3, scale)
	for i := 1; i <= steps; i++ {
		t := float32(i) / float32(steps)
		t2 := t * t
		t3 := t2 * t
		mt := 1 - t
		mt2 := mt * mt
		mt3 := mt2 * mt
		x := mt3*x0 + 3*mt2*t*x1 + 3*mt*t2*x2 + t3*x3
		y := mt3*y0 + 3*mt2*t*y1 + 3*mt*t2*y2 + t3*y3
		*out = append(*out, x*scale, y*scale)
	}
}

// flattenQuadratic approximates a quadratic Bezier with line
// segments. Step count adapts to chord length.
func flattenQuadratic(out *[]float32,
	x0, y0, x1, y1, x2, y2, scale float32) {
	steps := adaptiveSteps(x0, y0, x2, y2, scale)
	for i := 1; i <= steps; i++ {
		t := float32(i) / float32(steps)
		mt := 1 - t
		x := mt*mt*x0 + 2*mt*t*x1 + t*t*x2
		y := mt*mt*y0 + 2*mt*t*y1 + t*t*y2
		*out = append(*out, x*scale, y*scale)
	}
}

// adaptiveSteps computes the number of line segments for curve
// flattening based on the chord length between endpoints.
func adaptiveSteps(x0, y0, x1, y1, scale float32) int {
	dx := (x1 - x0) * scale
	dy := (y1 - y0) * scale
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	steps := int(dist / 4)
	if steps < 4 {
		return 4
	}
	if steps > 64 {
		return 64
	}
	return steps
}

// flattenArc approximates an SVG arc with line segments using
// endpoint parameterization.
func flattenArc(out *[]float32,
	cx, cy, rxIn, ryIn, rotDeg float32,
	largeArc, sweepFlag bool, x, y, scale float32) {
	// Degenerate: treat as line.
	if rxIn == 0 || ryIn == 0 {
		*out = append(*out, x*scale, y*scale)
		return
	}
	rx := float64(f32Abs(rxIn))
	ry := float64(f32Abs(ryIn))
	phi := float64(rotDeg) * math.Pi / 180

	dx := float64(cx-x) / 2
	dy := float64(cy-y) / 2
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Ensure radii are large enough.
	lambda := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lambda > 1 {
		sqrtL := math.Sqrt(lambda)
		rx *= sqrtL
		ry *= sqrtL
	}

	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	if den == 0 {
		*out = append(*out, x*scale, y*scale)
		return
	}
	sq := num / den
	sq = max(sq, 0)
	root := math.Sqrt(sq)
	if largeArc == sweepFlag {
		root = -root
	}
	cxp := root * rx * y1p / ry
	cyp := -root * ry * x1p / rx

	cxArc := cosPhi*cxp - sinPhi*cyp +
		float64(cx+x)/2
	cyArc := sinPhi*cxp + cosPhi*cyp +
		float64(cy+y)/2

	theta1 := vecAngle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dtheta := vecAngle(
		(x1p-cxp)/rx, (y1p-cyp)/ry,
		(-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweepFlag && dtheta > 0 {
		dtheta -= 2 * math.Pi
	}
	if sweepFlag && dtheta < 0 {
		dtheta += 2 * math.Pi
	}

	steps := int(math.Ceil(math.Abs(dtheta) / (math.Pi / 8)))
	steps = max(steps, 4)
	for i := 1; i <= steps; i++ {
		t := theta1 + dtheta*float64(i)/float64(steps)
		xr := rx * math.Cos(t)
		yr := ry * math.Sin(t)
		px := cosPhi*xr - sinPhi*yr + cxArc
		py := sinPhi*xr + cosPhi*yr + cyArc
		*out = append(*out, float32(px)*scale, float32(py)*scale)
	}
}

func vecAngle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	lenU := math.Sqrt(ux*ux + uy*uy)
	lenV := math.Sqrt(vx*vx + vy*vy)
	d := dot / (lenU * lenV)
	if d < -1 {
		d = -1
	}
	d = min(d, 1)
	a := math.Acos(d)
	if ux*vy-uy*vx < 0 {
		a = -a
	}
	return a
}

// buildArcLengthTable computes cumulative arc lengths along a
// polyline. Returns (table, totalLength). polyline is [x0,y0, ...].
func buildArcLengthTable(polyline []float32) ([]float32, float32) {
	n := len(polyline) / 2
	if n < 1 {
		return nil, 0
	}
	table := make([]float32, n)
	table[0] = 0
	for i := 1; i < n; i++ {
		dx := polyline[i*2] - polyline[(i-1)*2]
		dy := polyline[i*2+1] - polyline[(i-1)*2+1]
		table[i] = table[i-1] + float32(math.Sqrt(
			float64(dx*dx+dy*dy)))
	}
	return table, table[n-1]
}

// SamplePathAt returns (x, y, angle) at distance dist along the
// polyline. Uses binary search on the arc-length table.
func samplePathAt(polyline, table []float32,
	dist float32) (float32, float32, float32) {
	n := len(table)
	if n < 2 {
		if n == 1 {
			return polyline[0], polyline[1], 0
		}
		return 0, 0, 0
	}
	// Clamp before start.
	if dist <= 0 {
		dx := polyline[2] - polyline[0]
		dy := polyline[3] - polyline[1]
		return polyline[0], polyline[1],
			float32(math.Atan2(float64(dy), float64(dx)))
	}
	total := table[n-1]
	// Clamp beyond end.
	if dist >= total {
		last := (n - 1) * 2
		prev := (n - 2) * 2
		dx := polyline[last] - polyline[prev]
		dy := polyline[last+1] - polyline[prev+1]
		return polyline[last], polyline[last+1],
			float32(math.Atan2(float64(dy), float64(dx)))
	}
	// Binary search for enclosing segment.
	lo, hi := 0, n-1
	for lo < hi-1 {
		mid := (lo + hi) / 2
		if table[mid] <= dist {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Interpolate within segment.
	segLen := table[hi] - table[lo]
	t := float32(0)
	if segLen > 0 {
		t = (dist - table[lo]) / segLen
	}
	x0 := polyline[lo*2]
	y0 := polyline[lo*2+1]
	x1 := polyline[hi*2]
	y1 := polyline[hi*2+1]
	x := x0 + (x1-x0)*t
	y := y0 + (y1-y0)*t
	angle := float32(math.Atan2(float64(y1-y0), float64(x1-x0)))
	return x, y, angle
}
