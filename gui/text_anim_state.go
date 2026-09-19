package gui

import (
	"math"
	"strings"
	"time"

	"github.com/rivo/uniseg"
)

// Driver and per-text state for TextCfg.Anim: registering, restarting
// and retiring the keyframe animation that advances one animated text.
// The frame it produces is sampled and applied in text_anim.go.

// textAnimState is what one animated text keeps between frames.
//
// started and done replace a lookup of the driver. The loop deletes a
// finished one-shot at once, but its last OnValue and its OnDone only
// land at the next command flush, so a view pass in between finds no
// driver and a done flag that is still false. Reading "no driver" as
// "register one" replayed the entrance. Instead started records that a
// driver was registered: a started one-shot whose driver is gone has
// finished.
//
// gen tells the callbacks of a replaced driver from those of the live
// one. The deferred OnValue of a driver removed because the Cfg
// changed would otherwise write its progress into the new run.
//
// seen is the view pass that last generated this text, so entries for
// texts that left the tree can be dropped. See pruneTextAnimStates.
type textAnimState struct {
	// text is the string a typewriter run counts over, so a change to
	// it can be noticed and the reveal continued.
	text     string
	sig      textAnimSig
	seen     uint64
	progress float32
	gen      uint32
	// from is how many graphemes were already shown when this
	// typewriter run started. A run started by appended text begins
	// there, not at zero.
	from    int
	started bool
	done    bool
}

// textAnimSig is the part of a TextAnimCfg that picks the driver. A
// change to it restarts the animation. Easing is left out: it shapes
// the frame in the view pass and needs no new driver, and a func is
// not comparable anyway. Custom counts only as set or not, for the
// same reason.
type textAnimSig struct {
	dur    time.Duration
	delay  time.Duration
	kind   TextAnimKind
	custom bool
	repeat bool
}

func (a *TextAnimCfg) sig() textAnimSig {
	return textAnimSig{
		dur:    a.Duration,
		delay:  a.Delay,
		kind:   a.Kind,
		custom: a.Custom != nil,
		repeat: a.Repeat,
	}
}

// textAnimPruneAt is the entry count at which a new animated ID first
// drops the entries of texts that left the tree. The map is otherwise
// unbounded: a capped FIFO evicted the done flags of texts still on
// screen once more than its capacity were animated, and each evicted
// entrance played again, for ever.
const textAnimPruneAt = 256

// textAnimStates is the per-window map of animated text state. It is
// unbounded; pruneTextAnimStates keeps it to the texts in the tree.
func textAnimStates(w *Window) *BoundedMap[string, textAnimState] {
	return StateMap[string, textAnimState](w, nsTextAnim, 0)
}

// pruneTextAnimStates drops the entries of texts that were not
// generated in this view pass or the one before. A text in the tree
// is generated every pass, so an older entry is for a text that has
// gone; if it comes back, its entrance plays again, as for any new
// text. Rebuilding the map costs one allocation, and runs only when a
// new ID arrives at a full map.
func pruneTextAnimStates(pm *BoundedMap[string, textAnimState], pass uint64) {
	type kv struct {
		k string
		v textAnimState
	}
	keep := make([]kv, 0, pm.Len())
	pm.Range(func(k string, v textAnimState) bool {
		if v.seen+1 >= pass {
			keep = append(keep, kv{k, v})
		}
		return true
	})
	if len(keep) == pm.Len() {
		return
	}
	pm.Clear()
	for _, e := range keep {
		pm.Set(e.k, e.v)
	}
}

// syncTextAnimDriver brings the text's driver in line with its Cfg and
// returns the state to sample. It registers the driver on the first
// frame, restarts it when the Cfg or a typewriter's text changed,
// re-registers a loop whose text left the tree and came back, and
// reads a started one-shot whose driver is gone as finished.
//
// A finished one-shot with an unchanged Cfg costs a map read and a map
// write per frame: no driver lookup, no lock and no animation ID.
func syncTextAnimDriver(
	tv *textView, w *Window, key string,
) textAnimState {
	cfg := &tv.cfg.Anim
	pm := textAnimStates(w)
	st, ok := pm.Get(key)
	if !ok && pm.Len() >= textAnimPruneAt {
		pruneTextAnimStates(pm, w.viewPass)
	}

	sig := cfg.sig()
	typewriter := cfg.Custom == nil && cfg.Kind == TextAnimTypewriter
	cfgChanged := st.started && st.sig != sig
	textChanged := typewriter && st.started && st.text != tv.cfg.Text

	animID := ""
	if cfgChanged || textChanged {
		from := 0
		if !cfgChanged {
			from = textAnimTypedSoFar(&st, cfg, tv.cfg.Text)
		}
		animID = ScopeID("textanim", key)
		w.AnimationRemove(animID)
		st = textAnimState{gen: st.gen + 1, from: from}
	}
	st.sig = sig
	if typewriter {
		st.text = tv.cfg.Text
	}

	if !st.done {
		if animID == "" {
			animID = ScopeID("textanim", key)
		}
		register := !st.started
		if st.started && !w.touchViewBoundAnimation(animID) {
			if cfg.Repeat {
				// A loop never finishes, so a missing driver was
				// retired because its text left the tree for a while.
				register = true
			} else {
				// Finished: the loop removed the driver, and its last
				// OnValue and OnDone may not have been flushed yet.
				st.done = true
				st.progress = 1
			}
		}
		if register {
			dur := cfg.Duration
			if dur <= 0 {
				// Only the typewriter's default counts characters, so
				// only it pays for the O(n) count.
				chars := 0
				if cfg.Kind == TextAnimTypewriter {
					chars = uniseg.GraphemeClusterCount(tv.cfg.Text) -
						st.from
				}
				dur = textAnimDefaultDuration(cfg.Kind, chars)
			}
			w.animationAddViewBound(newTextAnimDriver(
				animID, key, st.gen, dur, cfg.Delay, cfg.Repeat,
			))
			st.started = true
		}
	}
	st.seen = w.viewPass
	pm.Set(key, st)
	return st
}

// textAnimTypedSoFar returns how many graphemes of next a typewriter
// can keep showing when its text changes from st.text to next. When
// next starts with what was already shown — text appended by a stream
// — the reveal carries on from there. Otherwise it starts over.
func textAnimTypedSoFar(
	st *textAnimState, cfg *TextAnimCfg, next string,
) int {
	total := uniseg.GraphemeClusterCount(st.text)
	shown := total
	if !st.done {
		easing := cfg.Easing
		if easing == nil {
			easing = textAnimDefaultEasing(cfg.Kind)
		}
		p := easing(st.progress)
		if !f32IsFinite(p) {
			return 0
		}
		shown = textAnimRevealCount(total, st.from, p)
	}
	prefix := st.text[:graphemePrefixBytes(st.text, shown)]
	if !strings.HasPrefix(next, prefix) {
		return 0
	}
	return shown
}

// newTextAnimDriver builds the keyframe animation that advances one
// animated text.
//
// The driver always produces linear progress; the caller eases it. A
// delay is expressed as a flat leading segment rather than as new
// machinery: the animation runs for delay+duration and holds 0 until
// the delay is spent.
//
// gen is the state generation the driver was registered under. Its
// callbacks write only while the state still carries that generation,
// and never re-create an entry that was pruned.
func newTextAnimDriver(
	animID, key string,
	gen uint32,
	dur, delay time.Duration,
	repeat bool,
) *KeyframeAnimation {
	// A negative delay would make total shorter than dur and put a
	// negative At into the keyframes, breaking the ascending-At
	// contract interpolateKeyframes depends on. Clamp it away.
	if delay < 0 {
		delay = 0
	}
	// A huge delay would overflow dur+delay into a negative total.
	// Cap it so the sum saturates at the largest Duration instead.
	if delay > math.MaxInt64-dur {
		delay = math.MaxInt64 - dur
	}
	total := dur + delay
	frames := []Keyframe{{At: 0, Value: 0}}
	if delay > 0 {
		frames = append(frames, Keyframe{
			At:    float32(delay) / float32(total),
			Value: 0,
		})
	}
	frames = append(frames, Keyframe{At: 1, Value: 1, Easing: EaseLinear})

	return &KeyframeAnimation{
		AnimID:    animID,
		Duration:  total,
		Repeat:    repeat,
		Keyframes: frames,
		OnValue: func(v float32, w *Window) {
			pm := textAnimStates(w)
			prev, ok := pm.Get(key)
			if !ok || prev.gen != gen {
				return
			}
			prev.progress = v
			pm.Set(key, prev)
		},
		OnDone: func(w *Window) {
			pm := textAnimStates(w)
			prev, ok := pm.Get(key)
			if !ok || prev.gen != gen {
				return
			}
			prev.done = true
			prev.progress = 1
			pm.Set(key, prev)
		},
	}
}
