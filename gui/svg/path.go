package svg

import (
	"math"
	"strconv"
)

// pathParser holds mutable state during SVG path d-attribute parsing.
type pathParser struct {
	tokens   []string
	i        int
	segments []PathSegment

	curX, curY           float32
	startX, startY       float32
	lastCtrlX, lastCtrlY float32
	lastCmd              byte
}

// parsePathD parses the SVG path d attribute into segments.
func parsePathD(d string) []PathSegment {
	p := pathParser{
		tokens:   tokenizePath(d),
		segments: make([]PathSegment, 0, 32),
	}
	return p.parse()
}

func (p *pathParser) parse() []PathSegment {
	for p.i < len(p.tokens) && len(p.segments) < maxPathSegments {
		token := p.tokens[p.i]
		if len(token) == 0 {
			p.i++
			continue
		}

		c := token[0]
		isCmd := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')

		// DoS guard. Captured before cmd-letter consumption so an
		// arg-less command (e.g. Z) followed by another command
		// (e.g. M) can't trip the post-switch skip and lose data.
		iBefore := p.i
		cmd := p.lastCmd
		if isCmd {
			cmd = c
			p.i++
		}

		cmd = p.dispatch(cmd)

		if p.i == iBefore {
			p.i++
		}
		p.lastCmd = cmd
	}
	return p.segments
}

func (p *pathParser) dispatch(cmd byte) byte {
	switch cmd {
	case 'M', 'm':
		return p.parseMoveTo(cmd)
	case 'L', 'l':
		return p.parseLineTo(cmd)
	case 'H', 'h':
		return p.parseHLineTo(cmd)
	case 'V', 'v':
		return p.parseVLineTo(cmd)
	case 'C', 'c':
		return p.parseCubicTo(cmd)
	case 'S', 's':
		return p.parseSmoothCubic(cmd)
	case 'Q', 'q':
		return p.parseQuadTo(cmd)
	case 'T', 't':
		return p.parseSmoothQuad(cmd)
	case 'A', 'a':
		return p.parseArcTo(cmd)
	case 'Z', 'z':
		return p.parseClose(cmd)
	default:
		p.i++
		return cmd
	}
}

func (p *pathParser) parseMoveTo(cmd byte) byte {
	relative := cmd == 'm'
	nextCmd := byte('L')
	if relative {
		nextCmd = 'l'
	}
	first := true
	for p.i < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		x := parseF32(p.tokens[p.i])
		y := float32(0)
		if p.i+1 < len(p.tokens) {
			y = parseF32(p.tokens[p.i+1])
		}
		p.i += 2
		if relative {
			p.curX += x
			p.curY += y
		} else {
			p.curX = x
			p.curY = y
		}
		if first {
			p.segments = append(p.segments, PathSegment{CmdMoveTo, []float32{p.curX, p.curY}})
			p.startX = p.curX
			p.startY = p.curY
			first = false
		} else {
			p.segments = append(p.segments, PathSegment{CmdLineTo, []float32{p.curX, p.curY}})
		}
	}
	if first {
		return cmd
	}
	return nextCmd
}

func (p *pathParser) parseLineTo(cmd byte) byte {
	relative := cmd == 'l'
	for p.i < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		x := parseF32(p.tokens[p.i])
		y := float32(0)
		if p.i+1 < len(p.tokens) {
			y = parseF32(p.tokens[p.i+1])
		}
		p.i += 2
		if relative {
			p.curX += x
			p.curY += y
		} else {
			p.curX = x
			p.curY = y
		}
		p.segments = append(p.segments, PathSegment{CmdLineTo, []float32{p.curX, p.curY}})
	}
	return cmd
}

func (p *pathParser) parseHLineTo(cmd byte) byte {
	relative := cmd == 'h'
	for p.i < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		x := parseF32(p.tokens[p.i])
		p.i++
		if relative {
			p.curX += x
		} else {
			p.curX = x
		}
		p.segments = append(p.segments, PathSegment{CmdLineTo, []float32{p.curX, p.curY}})
	}
	return cmd
}

func (p *pathParser) parseVLineTo(cmd byte) byte {
	relative := cmd == 'v'
	for p.i < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		y := parseF32(p.tokens[p.i])
		p.i++
		if relative {
			p.curY += y
		} else {
			p.curY = y
		}
		p.segments = append(p.segments, PathSegment{CmdLineTo, []float32{p.curX, p.curY}})
	}
	return cmd
}

func (p *pathParser) parseCubicTo(cmd byte) byte {
	relative := cmd == 'c'
	for p.i+5 < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		c1x := parseF32(p.tokens[p.i])
		c1y := parseF32(p.tokens[p.i+1])
		c2x := parseF32(p.tokens[p.i+2])
		c2y := parseF32(p.tokens[p.i+3])
		x := parseF32(p.tokens[p.i+4])
		y := parseF32(p.tokens[p.i+5])
		p.i += 6
		if relative {
			p.segments = append(p.segments, PathSegment{CmdCubicTo, []float32{
				p.curX + c1x, p.curY + c1y,
				p.curX + c2x, p.curY + c2y,
				p.curX + x, p.curY + y,
			}})
			p.lastCtrlX = p.curX + c2x
			p.lastCtrlY = p.curY + c2y
			p.curX += x
			p.curY += y
		} else {
			p.segments = append(p.segments, PathSegment{CmdCubicTo, []float32{
				c1x, c1y, c2x, c2y, x, y,
			}})
			p.lastCtrlX = c2x
			p.lastCtrlY = c2y
			p.curX = x
			p.curY = y
		}
	}
	return cmd
}

func (p *pathParser) parseSmoothCubic(cmd byte) byte {
	relative := cmd == 's'
	for p.i+3 < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		var c1x, c1y float32
		if p.lastCmd == 'C' || p.lastCmd == 'c' || p.lastCmd == 'S' || p.lastCmd == 's' {
			c1x = p.curX*2 - p.lastCtrlX
			c1y = p.curY*2 - p.lastCtrlY
		} else {
			c1x = p.curX
			c1y = p.curY
		}
		c2x := parseF32(p.tokens[p.i])
		c2y := parseF32(p.tokens[p.i+1])
		x := parseF32(p.tokens[p.i+2])
		y := parseF32(p.tokens[p.i+3])
		p.i += 4
		if relative {
			p.segments = append(p.segments, PathSegment{CmdCubicTo, []float32{
				c1x, c1y,
				p.curX + c2x, p.curY + c2y,
				p.curX + x, p.curY + y,
			}})
			p.lastCtrlX = p.curX + c2x
			p.lastCtrlY = p.curY + c2y
			p.curX += x
			p.curY += y
		} else {
			p.segments = append(p.segments, PathSegment{CmdCubicTo, []float32{
				c1x, c1y, c2x, c2y, x, y,
			}})
			p.lastCtrlX = c2x
			p.lastCtrlY = c2y
			p.curX = x
			p.curY = y
		}
		p.lastCmd = cmd
	}
	return cmd
}

func (p *pathParser) parseQuadTo(cmd byte) byte {
	relative := cmd == 'q'
	for p.i+3 < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		cx := parseF32(p.tokens[p.i])
		cy := parseF32(p.tokens[p.i+1])
		x := parseF32(p.tokens[p.i+2])
		y := parseF32(p.tokens[p.i+3])
		p.i += 4
		if relative {
			p.segments = append(p.segments, PathSegment{CmdQuadTo, []float32{
				p.curX + cx, p.curY + cy,
				p.curX + x, p.curY + y,
			}})
			p.lastCtrlX = p.curX + cx
			p.lastCtrlY = p.curY + cy
			p.curX += x
			p.curY += y
		} else {
			p.segments = append(p.segments, PathSegment{CmdQuadTo, []float32{
				cx, cy, x, y,
			}})
			p.lastCtrlX = cx
			p.lastCtrlY = cy
			p.curX = x
			p.curY = y
		}
	}
	return cmd
}

func (p *pathParser) parseSmoothQuad(cmd byte) byte {
	relative := cmd == 't'
	for p.i+1 < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		var cx, cy float32
		if p.lastCmd == 'Q' || p.lastCmd == 'q' || p.lastCmd == 'T' || p.lastCmd == 't' {
			cx = p.curX*2 - p.lastCtrlX
			cy = p.curY*2 - p.lastCtrlY
		} else {
			cx = p.curX
			cy = p.curY
		}
		x := parseF32(p.tokens[p.i])
		y := parseF32(p.tokens[p.i+1])
		p.i += 2
		if relative {
			p.segments = append(p.segments, PathSegment{CmdQuadTo, []float32{
				cx, cy, p.curX + x, p.curY + y,
			}})
			p.lastCtrlX = cx
			p.lastCtrlY = cy
			p.curX += x
			p.curY += y
		} else {
			p.segments = append(p.segments, PathSegment{CmdQuadTo, []float32{
				cx, cy, x, y,
			}})
			p.lastCtrlX = cx
			p.lastCtrlY = cy
			p.curX = x
			p.curY = y
		}
		p.lastCmd = cmd
	}
	return cmd
}

func (p *pathParser) parseArcTo(cmd byte) byte {
	relative := cmd == 'a'
	for p.i+6 < len(p.tokens) && isNumberToken(p.tokens[p.i]) {
		rx := parseF32(p.tokens[p.i])
		ry := parseF32(p.tokens[p.i+1])
		phi := parseF32(p.tokens[p.i+2])
		largeArc := parseF32(p.tokens[p.i+3]) != 0
		sweep := parseF32(p.tokens[p.i+4]) != 0
		x := parseF32(p.tokens[p.i+5])
		y := parseF32(p.tokens[p.i+6])
		p.i += 7

		ex, ey := x, y
		if relative {
			ex += p.curX
			ey += p.curY
		}

		if rx <= 0 || ry <= 0 {
			p.segments = append(p.segments, PathSegment{CmdLineTo, []float32{ex, ey}})
		} else {
			arcSegs := arcToCubic(p.curX, p.curY, rx, ry, phi, largeArc, sweep, ex, ey)
			p.segments = append(p.segments, arcSegs...)
		}
		p.curX = ex
		p.curY = ey
	}
	return cmd
}

func (p *pathParser) parseClose(cmd byte) byte {
	p.segments = append(p.segments, PathSegment{CmdClose, nil})
	p.curX = p.startX
	p.curY = p.startY
	return cmd
}

// arcToCubic converts an SVG arc to cubic bezier curves.
func arcToCubic(x1, y1, rx, ry, phi float32, largeArc, sweep bool, x2, y2 float32) []PathSegment {
	if rx == 0 || ry == 0 {
		return []PathSegment{{CmdLineTo, []float32{x2, y2}}}
	}

	rxAbs := f32Abs(rx)
	ryAbs := f32Abs(ry)
	phiRad := float64(phi) * math.Pi / 180.0

	cosPhi := float32(math.Cos(phiRad))
	sinPhi := float32(math.Sin(phiRad))

	dx := (x1 - x2) / 2
	dy := (y1 - y2) / 2
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Correct radii
	lambda := (x1p*x1p)/(rxAbs*rxAbs) + (y1p*y1p)/(ryAbs*ryAbs)
	if lambda > 1 {
		sqrtLambda := float32(math.Sqrt(float64(lambda)))
		rxAbs *= sqrtLambda
		ryAbs *= sqrtLambda
	}

	rx2 := rxAbs * rxAbs
	ry2 := ryAbs * ryAbs
	x1p2 := x1p * x1p
	y1p2 := y1p * y1p

	sq := (rx2*ry2 - rx2*y1p2 - ry2*x1p2) / (rx2*y1p2 + ry2*x1p2)
	sq = max(sq, 0)
	coef := float32(math.Sqrt(float64(sq)))
	if largeArc == sweep {
		coef = -coef
	}

	cxp := coef * rxAbs * y1p / ryAbs
	cyp := -coef * ryAbs * x1p / rxAbs

	cx := cosPhi*cxp - sinPhi*cyp + (x1+x2)/2
	cy := sinPhi*cxp + cosPhi*cyp + (y1+y2)/2

	theta1 := vectorAngle(1, 0, (x1p-cxp)/rxAbs, (y1p-cyp)/ryAbs)
	dtheta := vectorAngle((x1p-cxp)/rxAbs, (y1p-cyp)/ryAbs, (-x1p-cxp)/rxAbs, (-y1p-cyp)/ryAbs)

	if !sweep && dtheta > 0 {
		dtheta -= 2 * math.Pi
	} else if sweep && dtheta < 0 {
		dtheta += 2 * math.Pi
	}

	nSegs := int(math.Ceil(math.Abs(float64(dtheta)) / (math.Pi / 2)))
	dTheta := dtheta / float32(nSegs)

	segments := make([]PathSegment, 0, nSegs)
	theta := theta1
	for range nSegs {
		seg := arcSegmentToCubic(cx, cy, rxAbs, ryAbs, float32(phiRad), theta, dTheta)
		segments = append(segments, seg)
		theta += dTheta
	}
	return segments
}

func vectorAngle(ux, uy, vx, vy float32) float32 {
	n := float32(math.Sqrt(float64(ux*ux+uy*uy))) * float32(math.Sqrt(float64(vx*vx+vy*vy)))
	if n == 0 {
		return 0
	}
	c := (ux*vx + uy*vy) / n
	if c < -1 {
		c = -1
	}
	c = min(c, 1)
	angle := float32(math.Acos(float64(c)))
	if ux*vy-uy*vx < 0 {
		return -angle
	}
	return angle
}

func arcSegmentToCubic(cx, cy, rx, ry, phi, theta, dtheta float32) PathSegment {
	t := float32(math.Tan(float64(dtheta/4))) * 4 / 3

	cosTheta := float32(math.Cos(float64(theta)))
	sinTheta := float32(math.Sin(float64(theta)))
	cosTheta2 := float32(math.Cos(float64(theta + dtheta)))
	sinTheta2 := float32(math.Sin(float64(theta + dtheta)))

	cosPhi := float32(math.Cos(float64(phi)))
	sinPhi := float32(math.Sin(float64(phi)))

	x1 := rx * cosTheta
	y1 := ry * sinTheta
	dx1 := -rx * sinTheta * t
	dy1 := ry * cosTheta * t

	x2 := rx * cosTheta2
	y2 := ry * sinTheta2
	dx2 := -rx * sinTheta2 * t
	dy2 := ry * cosTheta2 * t

	p1x := cosPhi*(x1+dx1) - sinPhi*(y1+dy1) + cx
	p1y := sinPhi*(x1+dx1) + cosPhi*(y1+dy1) + cy
	p2x := cosPhi*(x2-dx2) - sinPhi*(y2-dy2) + cx
	p2y := sinPhi*(x2-dx2) + cosPhi*(y2-dy2) + cy
	ex := cosPhi*x2 - sinPhi*y2 + cx
	ey := sinPhi*x2 + cosPhi*y2 + cy

	return PathSegment{CmdCubicTo, []float32{p1x, p1y, p2x, p2y, ex, ey}}
}

// tokenizePath splits path d string into tokens.
//
//nolint:gocyclo // character-level tokenizer
func tokenizePath(d string) []string {
	tokens := make([]string, 0, len(d)/4+1)
	tokenStart := -1
	hasDot := false
	lastByte := byte(0)

	flushCurrent := func(end int) bool {
		if tokenStart < 0 {
			return true
		}
		tokens = append(tokens, d[tokenStart:end])
		tokenStart = -1
		hasDot = false
		lastByte = 0
		return len(tokens) < maxPathSegments
	}

	for i := range len(d) {
		if len(tokens) >= maxPathSegments {
			break
		}
		c := d[i]

		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			if !flushCurrent(i) {
				break
			}
			continue
		}

		// Command letters
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			if (c == 'e' || c == 'E') && tokenStart >= 0 {
				lastByte = c
				continue
			}
			if !flushCurrent(i) {
				break
			}
			tokens = append(tokens, d[i:i+1])
			continue
		}

		// Numbers
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' {
			if (c == '-' || c == '+') && tokenStart >= 0 {
				if lastByte != 'e' && lastByte != 'E' {
					if !flushCurrent(i) {
						break
					}
				}
			}
			if c == '.' && hasDot && tokenStart >= 0 {
				if !flushCurrent(i) {
					break
				}
			}
			if tokenStart < 0 {
				tokenStart = i
			}
			if c == '.' {
				hasDot = true
			}
			lastByte = c
			continue
		}
	}

	flushCurrent(len(d))
	return tokens
}

func isNumberToken(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[0]
	return (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.'
}

// parseF32 silently zeros for malformed/NaN/Inf input. Reserved for
// tolerant geometry paths; SMIL endpoints use parseFloatStrict so a
// bogus 0 cannot synthesize a real animation endpoint.
func parseF32(s string) float32 {
	v, _ := strconv.ParseFloat(s, 32)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return float32(v)
}

// parseNumberList parses a space/comma separated list of numbers.
func parseNumberList(s string) []float32 {
	tokens := tokenizePath(s)
	numbers := make([]float32, 0, len(tokens))
	for _, t := range tokens {
		if isNumberToken(t) {
			numbers = append(numbers, parseF32(t))
		}
	}
	return numbers
}
