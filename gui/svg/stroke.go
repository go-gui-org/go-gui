package svg

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// tessellateStroke converts polylines to stroke triangles.
func tessellateStroke(polylines [][]float32, width float32, lineCap gui.StrokeCap, join gui.StrokeJoin) []float32 {
	result := make([]float32, 0, estimateStrokeResultCap(polylines, lineCap, join))
	halfW := width / 2

	for _, poly := range polylines {
		if len(poly) < 4 || len(poly)%2 != 0 {
			continue
		}
		n := len(poly) / 2
		if n < 2 {
			continue
		}

		// Check if closed
		dxClose := poly[0] - poly[(n-1)*2]
		dyClose := poly[1] - poly[(n-1)*2+1]
		isClosed := n > 2 && f32Abs(dxClose) < closedPathEpsilon &&
			f32Abs(dyClose) < closedPathEpsilon
		pointCount := n
		if isClosed {
			pointCount = n - 1
		}
		if pointCount < 2 {
			continue
		}

		// Build normals
		normals := make([]float32, 0, pointCount*2)
		for i := 0; i < pointCount-1; i++ {
			dx := poly[(i+1)*2] - poly[i*2]
			dy := poly[(i+1)*2+1] - poly[i*2+1]
			l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if l > 0 {
				normals = append(normals, -dy/l, dx/l)
			} else {
				normals = append(normals, 0, 1)
			}
		}
		if isClosed {
			dx := poly[0] - poly[(pointCount-1)*2]
			dy := poly[1] - poly[(pointCount-1)*2+1]
			l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if l > 0 {
				normals = append(normals, -dy/l, dx/l)
			} else {
				normals = append(normals, 0, 1)
			}
		}

		// Generate stroke quads. Closed polylines emit an extra
		// segment wrapping the last point back to the first; without
		// it the stroke leaves a visible gap (e.g. circle outlines
		// in spinning-circles.svg appearing broken).
		segCount := pointCount - 1
		if isClosed {
			segCount = pointCount
		}
		for i := 0; i < segCount; i++ {
			i0 := i
			i1 := i + 1
			if i1 == pointCount {
				i1 = 0
			}
			x0 := poly[i0*2]
			y0 := poly[i0*2+1]
			x1 := poly[i1*2]
			y1 := poly[i1*2+1]
			nx := normals[i*2]
			ny := normals[i*2+1]

			ax := x0 + nx*halfW
			ay := y0 + ny*halfW
			bx := x0 - nx*halfW
			by := y0 - ny*halfW
			cx := x1 - nx*halfW
			cy := y1 - ny*halfW
			dx := x1 + nx*halfW
			dy := y1 + ny*halfW

			result = append(result, ax, ay, bx, by, cx, cy, ax, ay, cx, cy, dx, dy)
		}

		// Line joins
		numNormals := len(normals) / 2
		if isClosed {
			for i := range pointCount {
				prevNorm := i - 1
				if i == 0 {
					prevNorm = numNormals - 1
				}
				nextNorm := i
				if nextNorm < numNormals && prevNorm < numNormals {
					addLineJoin(poly[i*2], poly[i*2+1],
						normals[prevNorm*2], normals[prevNorm*2+1],
						normals[nextNorm*2], normals[nextNorm*2+1],
						halfW, join, &result)
				}
			}
		} else {
			for i := 1; i < pointCount-1; i++ {
				if i < numNormals {
					addLineJoin(poly[i*2], poly[i*2+1],
						normals[(i-1)*2], normals[(i-1)*2+1],
						normals[i*2], normals[i*2+1],
						halfW, join, &result)
				}
			}
		}

		// Line caps (open paths only)
		if !isClosed && len(normals) >= 2 {
			addLineCap(poly[0], poly[1],
				-normals[0], -normals[1], normals[0], normals[1],
				halfW, lineCap, &result)
			lastIdx := (pointCount - 1) * 2
			lastNormIdx := (len(normals)/2 - 1) * 2
			addLineCap(poly[lastIdx], poly[lastIdx+1],
				normals[lastNormIdx], normals[lastNormIdx+1],
				-normals[lastNormIdx], -normals[lastNormIdx+1],
				halfW, lineCap, &result)
		}
	}
	return result
}

func estimateStrokeResultCap(polylines [][]float32, lineCap gui.StrokeCap, join gui.StrokeJoin) int {
	total := 0
	for _, poly := range polylines {
		if len(poly) < 4 || len(poly)%2 != 0 {
			continue
		}
		n := len(poly) / 2
		if n < 2 {
			continue
		}
		pointCount := n
		dxClose := poly[0] - poly[(n-1)*2]
		dyClose := poly[1] - poly[(n-1)*2+1]
		isClosed := n > 2 && f32Abs(dxClose) < closedPathEpsilon &&
			f32Abs(dyClose) < closedPathEpsilon
		if isClosed {
			pointCount--
		}
		if pointCount < 2 {
			continue
		}
		segCount := pointCount - 1
		joinCount := pointCount - 2
		if isClosed {
			segCount = pointCount
			joinCount = pointCount
		}
		total += segCount * 12
		switch join {
		case gui.MiterJoin, gui.BevelJoin:
			total += joinCount * 6
		case gui.RoundJoin:
			total += joinCount * strokeRoundCapSegs * 6
		}
		if !isClosed {
			switch lineCap {
			case gui.SquareCap:
				total += 2 * 12
			case gui.RoundCap:
				total += 2 * strokeRoundCapSegs * 6
			}
		}
	}
	return total
}

func addLineJoin(x, y, n1x, n1y, n2x, n2y, halfW float32, join gui.StrokeJoin, result *[]float32) {
	cross := n1x*n2y - n1y*n2x
	if f32Abs(cross) < strokeCrossTolerance {
		return
	}

	dot := n1x*n2x + n1y*n2y
	miterLen := halfW / float32(math.Sqrt(float64((1+dot)/2)))
	miterLimit := strokeMiterLimit * halfW

	mx := n1x + n2x
	my := n1y + n2y
	mlen := float32(math.Sqrt(float64(mx*mx + my*my)))
	if mlen <= 0 {
		return
	}

	mxN := mx / mlen
	myN := my / mlen

	if join == gui.MiterJoin && miterLen <= miterLimit {
		if cross > 0 {
			*result = append(*result,
				x, y, x-n1x*halfW, y-n1y*halfW, x-mxN*miterLen, y-myN*miterLen,
				x, y, x-mxN*miterLen, y-myN*miterLen, x-n2x*halfW, y-n2y*halfW,
			)
		} else {
			*result = append(*result,
				x, y, x+mxN*miterLen, y+myN*miterLen, x+n1x*halfW, y+n1y*halfW,
				x, y, x+n2x*halfW, y+n2y*halfW, x+mxN*miterLen, y+myN*miterLen,
			)
		}
	} else if join == gui.RoundJoin {
		addRoundJoin(x, y, n1x, n1y, n2x, n2y, halfW, cross > 0, result)
	} else {
		// Bevel
		if cross > 0 {
			*result = append(*result,
				x, y, x-n1x*halfW, y-n1y*halfW, x-n2x*halfW, y-n2y*halfW,
			)
		} else {
			*result = append(*result,
				x, y, x+n2x*halfW, y+n2y*halfW, x+n1x*halfW, y+n1y*halfW,
			)
		}
	}
}

func addRoundJoin(x, y, n1x, n1y, n2x, n2y, halfW float32, leftTurn bool, result *[]float32) {
	angle1 := float32(math.Atan2(float64(n1y), float64(n1x)))
	angle2 := float32(math.Atan2(float64(n2y), float64(n2x)))

	if leftTurn {
		if angle2 > angle1 {
			angle2 -= 2 * math.Pi
		}
	} else {
		if angle2 < angle1 {
			angle2 += 2 * math.Pi
		}
	}

	angleDiff := f32Abs(angle2 - angle1)
	segments := int(math.Ceil(float64(angleDiff)/(math.Pi/4))) + 1
	if segments < 2 {
		return
	}

	step := (angle2 - angle1) / float32(segments)
	sign := float32(1)
	if leftTurn {
		sign = -1
	}
	prevX := x + float32(math.Cos(float64(angle1)))*halfW*sign
	prevY := y + float32(math.Sin(float64(angle1)))*halfW*sign

	for i := 1; i <= segments; i++ {
		angle := angle1 + step*float32(i)
		currX := x + float32(math.Cos(float64(angle)))*halfW*sign
		currY := y + float32(math.Sin(float64(angle)))*halfW*sign

		if leftTurn {
			*result = append(*result, x, y, prevX, prevY, currX, currY)
		} else {
			*result = append(*result, x, y, currX, currY, prevX, prevY)
		}
		prevX = currX
		prevY = currY
	}
}

func addLineCap(x, y, dx, dy, nx, ny, halfW float32, lineCap gui.StrokeCap, result *[]float32) {
	if lineCap == gui.ButtCap {
		return
	}

	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if l == 0 {
		return
	}
	dirX := dx / l
	dirY := dy / l

	switch lineCap {
	case gui.SquareCap:
		ex := x + dirX*halfW
		ey := y + dirY*halfW
		ax := x + nx*halfW
		ay := y + ny*halfW
		bx := x - nx*halfW
		by := y - ny*halfW
		cx := ex - nx*halfW
		cy := ey - ny*halfW
		dx2 := ex + nx*halfW
		dy2 := ey + ny*halfW
		*result = append(*result, ax, ay, bx, by, cx, cy, ax, ay, cx, cy, dx2, dy2)
	case gui.RoundCap:
		startAngle := float32(math.Atan2(float64(ny), float64(nx)))
		prevX := x + float32(math.Cos(float64(startAngle)))*halfW
		prevY := y + float32(math.Sin(float64(startAngle)))*halfW

		for i := 1; i <= strokeRoundCapSegs; i++ {
			angle := startAngle + math.Pi*float32(i)/float32(strokeRoundCapSegs)
			currX := x + float32(math.Cos(float64(angle)))*halfW
			currY := y + float32(math.Sin(float64(angle)))*halfW
			*result = append(*result, x, y, prevX, prevY, currX, currY)
			prevX = currX
			prevY = currY
		}
	}
}
