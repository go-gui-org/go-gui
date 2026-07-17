package main

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	historyWindow = 60 * time.Second
	// 2s buckets over a 60s window give 30 bars across the panel — wide enough
	// to read as bars rather than hairlines. Each bucket still averages its
	// samples, so 1s sampling just puts two samples per bar.
	historyBucket = 2 * time.Second

	chartWidth  float32 = 340
	chartBarsH  float32 = 56
	barMinPixel float32 = 1
)

// HistBucket is one fixed time slot of a resampled history series.
type HistBucket struct {
	Value        float64
	HasData      bool
	Interpolated bool
}

// usageChart renders one fixed-window history chart as a row of container
// bars — no canvas, no chart library. valueFn selects the metric; minScale is
// the lowest the y-axis top may be (CPU pins at 100, RAM floats to the max
// seen); fill is the bar color; fmtFn formats the scale readout.
func usageChart(
	p *Process,
	title string,
	minScale float64,
	fill gui.Color,
	valueFn func(ProcessPoint) float64,
	fmtFn func(float64) string,
) gui.View {
	theme := gui.CurrentTheme()
	buckets := resampleHistory(p.History, historyWindow, historyBucket, valueFn)

	// The y-axis top is the larger of minScale and the biggest real sample.
	scale := minScale
	for _, b := range buckets {
		if b.HasData && b.Value > scale {
			scale = b.Value
		}
	}
	if scale <= 0 {
		scale = 1
	}

	fillDim := fill
	fillDim.A = 120 // interpolated slots draw lighter

	bars := make([]gui.View, 0, len(buckets))
	for _, b := range buckets {
		bars = append(bars, bucketBar(b, scale, fill, fillDim, theme))
	}

	return gui.Column(gui.ContainerCfg{
		Width:   chartWidth,
		Sizing:  gui.FixedFit,
		Spacing: gui.SomeF(4),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SomeF(8),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: title, TextStyle: theme.B5}),
					gui.Text(gui.TextCfg{Text: "scale " + fmtFn(scale), TextStyle: theme.N6}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Width:   chartWidth,
				Height:  chartBarsH,
				Sizing:  gui.FixedFixed,
				Clip:    true,
				Radius:  gui.SomeF(theme.RadiusSmall),
				Color:   theme.ColorInterior,
				Padding: gui.SomeP(4, 4, 4, 4),
				Spacing: gui.SomeF(1),
				Content: bars,
			}),
		},
	})
}

// bucketBar renders one time slot: a bottom-anchored bar sized to the metric,
// or a faint baseline when the slot has no data.
func bucketBar(b HistBucket, scale float64, fill, fillDim gui.Color, theme gui.Theme) gui.View {
	if !b.HasData {
		return gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignBottom,
			Content: []gui.View{
				gui.Rectangle(gui.RectangleCfg{
					Height: barMinPixel,
					Sizing: gui.FillFixed,
					Color:  theme.ColorBorder,
				}),
			},
		})
	}

	ratio := b.Value / scale
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	barH := max(barMinPixel, float32(ratio)*(chartBarsH-8))
	color := fill
	if b.Interpolated {
		color = fillDim
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignBottom,
		Content: []gui.View{
			gui.Rectangle(gui.RectangleCfg{
				Height: barH,
				Sizing: gui.FillFixed,
				Color:  color,
				Radius: 1,
			}),
		},
	})
}

// resampleHistory buckets raw, irregularly-spaced samples into a fixed number
// of equal-duration time slots. The rightmost slot ends at the latest sample;
// slots march left into the past; samples in the same slot are averaged; empty
// slots between two real slots are linearly interpolated so slow sampling still
// draws a continuous line. Slots before the first real sample stay empty rather
// than inventing data.
func resampleHistory(hist []ProcessPoint, window, bucket time.Duration, valueFn func(ProcessPoint) float64) []HistBucket {
	n := max(int(window/bucket), 1)
	buckets := make([]HistBucket, n)
	if len(hist) == 0 {
		return buckets
	}

	// Bucket on fixed wall-clock boundaries so historical bars do not shift as
	// sub-second samples slide the latest timestamp forward each redraw.
	bucketNS := bucket.Nanoseconds()
	if bucketNS <= 0 {
		bucketNS = int64(time.Second)
	}
	latestBucket := hist[len(hist)-1].Time.UnixNano() / bucketNS
	sums := make([]float64, n)
	counts := make([]int, n)
	for _, pt := range hist {
		ptBucket := pt.Time.UnixNano() / bucketNS
		fromRight := max(latestBucket-ptBucket, 0)
		idx := n - 1 - int(fromRight)
		if idx < 0 || idx >= n {
			continue
		}
		sums[idx] += valueFn(pt)
		counts[idx]++
	}
	for i := range buckets {
		if counts[i] > 0 {
			buckets[i].Value = sums[i] / float64(counts[i])
			buckets[i].HasData = true
		}
	}

	// Linearly fill gaps between real slots.
	prev := -1
	for i := range n {
		if !buckets[i].HasData {
			continue
		}
		if prev >= 0 && i-prev > 1 {
			for k := prev + 1; k < i; k++ {
				t := float64(k-prev) / float64(i-prev)
				buckets[k].Value = buckets[prev].Value + (buckets[i].Value-buckets[prev].Value)*t
				buckets[k].HasData = true
				buckets[k].Interpolated = true
			}
		}
		prev = i
	}
	return buckets
}

// ramHistoryScale rounds the max RSS in history up to a 500 MiB step so the RAM
// chart's y-axis is stable and readable.
func ramHistoryScale(hist []ProcessPoint) float64 {
	const step = 500 * 1024 * 1024
	var maxRSS uint64
	for _, pt := range hist {
		if pt.RSSBytes > maxRSS {
			maxRSS = pt.RSSBytes
		}
	}
	if maxRSS == 0 {
		return float64(step)
	}
	scale := ((maxRSS + step - 1) / step) * step
	return float64(scale)
}
