package svg

import (
	"math"
	"strings"
)

// --- Transform parsing ---

// matrixMultiply composes two affine transforms: result = m1 * m2.
func matrixMultiply(m1, m2 [6]float32) [6]float32 {
	return [6]float32{
		m1[0]*m2[0] + m1[2]*m2[1],
		m1[1]*m2[0] + m1[3]*m2[1],
		m1[0]*m2[2] + m1[2]*m2[3],
		m1[1]*m2[2] + m1[3]*m2[3],
		m1[0]*m2[4] + m1[2]*m2[5] + m1[4],
		m1[1]*m2[4] + m1[3]*m2[5] + m1[5],
	}
}

// parseTransform parses SVG transform attribute.
func parseTransform(s string) [6]float32 {
	result := identityTransform
	str := strings.TrimSpace(s)
	pos := 0
	count := 0

	for pos < len(str) {
		count++
		if count > 100 {
			break
		}
		// Skip whitespace and commas
		for pos < len(str) && (str[pos] == ' ' || str[pos] == ',' || str[pos] == '\t') {
			pos++
		}
		if pos >= len(str) {
			break
		}
		// Find transform name
		nameEnd := pos
		for nameEnd < len(str) && str[nameEnd] != '(' && str[nameEnd] != ' ' {
			nameEnd++
		}
		name := str[pos:nameEnd]

		parenStart := strings.IndexByte(str[nameEnd:], '(')
		if parenStart < 0 {
			break
		}
		parenStart += nameEnd
		parenEnd := strings.IndexByte(str[parenStart:], ')')
		if parenEnd < 0 {
			break
		}
		parenEnd += parenStart

		args := parseNumberList(str[parenStart+1 : parenEnd])
		m := parseSingleTransform(name, args)
		result = matrixMultiply(result, m)
		pos = parenEnd + 1
	}
	return result
}

func parseSingleTransform(name string, args []float32) [6]float32 {
	switch name {
	case "matrix":
		if len(args) >= 6 {
			return [6]float32{args[0], args[1], args[2], args[3], args[4], args[5]}
		}
	case "translate":
		tx := float32(0)
		ty := float32(0)
		if len(args) >= 1 {
			tx = args[0]
		}
		if len(args) >= 2 {
			ty = args[1]
		}
		return [6]float32{1, 0, 0, 1, tx, ty}
	case "scale":
		sx := float32(1)
		sy := sx
		if len(args) >= 1 {
			sx = args[0]
			sy = sx
		}
		if len(args) >= 2 {
			sy = args[1]
		}
		return [6]float32{sx, 0, 0, sy, 0, 0}
	case "rotate":
		return parseRotateTransform(args)
	case "skewX":
		if len(args) >= 1 {
			angle := args[0] * math.Pi / 180.0
			return [6]float32{1, 0, float32(math.Tan(float64(angle))), 1, 0, 0}
		}
	case "skewY":
		if len(args) >= 1 {
			angle := args[0] * math.Pi / 180.0
			return [6]float32{1, float32(math.Tan(float64(angle))), 0, 1, 0, 0}
		}
	}
	return identityTransform
}

func parseRotateTransform(args []float32) [6]float32 {
	if len(args) < 1 {
		return identityTransform
	}
	angle := float64(args[0]) * math.Pi / 180.0
	cosA := float32(math.Cos(angle))
	sinA := float32(math.Sin(angle))
	if len(args) >= 3 {
		cx := args[1]
		cy := args[2]
		return [6]float32{
			cosA, sinA, -sinA, cosA,
			cx - cosA*cx + sinA*cy,
			cy - sinA*cx - cosA*cy,
		}
	}
	return [6]float32{cosA, sinA, -sinA, cosA, 0, 0}
}

// applyTransformPt transforms a point by affine matrix.
func applyTransformPt(x, y float32, m [6]float32) (float32, float32) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// applyInheritedTransformPt transforms (x, y) by m when m is an
// "active" matrix. Both the SVG identity matrix and a fully-zero
// matrix are treated as no-ops: identity because the transform is
// a no-op by definition, zero because tests construct ComputedStyle{}
// directly and the resulting zero matrix would otherwise collapse
// every point to the origin.
func applyInheritedTransformPt(x, y float32, m [6]float32) (float32, float32) {
	if m == identityTransform {
		return x, y
	}
	if m[0] == 0 && m[1] == 0 && m[2] == 0 && m[3] == 0 && m[4] == 0 && m[5] == 0 {
		return x, y
	}
	return applyTransformPt(x, y, m)
}

func isIdentityTransform(m [6]float32) bool {
	return m[0] == 1 && m[1] == 0 && m[2] == 0 && m[3] == 1 && m[4] == 0 && m[5] == 0
}
