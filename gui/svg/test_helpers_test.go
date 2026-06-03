package svg

import "github.com/go-gui-org/go-gui/gui"

// firstAnimatedPathID returns the PathID of the first animated
// tessellated path in parsed, or 0 if none.
func firstAnimatedPathID(parsed *gui.SvgParsed) uint32 {
	for _, tp := range parsed.Paths {
		if tp.Animated {
			return tp.PathID
		}
	}
	return 0
}
