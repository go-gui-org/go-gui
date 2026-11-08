package gui

import (
	"runtime"
	"sync"
	"testing"
)

// serialOf returns the serial LookupImage reports for src, failing the
// test when src is not registered.
func serialOf(t *testing.T, src string) uint64 {
	t.Helper()
	mi, ok := LookupImage(src)
	if !ok {
		t.Fatalf("LookupImage(%q): not registered", src)
	}
	if mi.Serial == 0 {
		t.Fatalf("LookupImage(%q): serial 0", src)
	}
	return mi.Serial
}

// A serial names one registration: repeating it keeps the serial, and
// every way a key comes back as a new registration gives a new one.
func TestImageSerialNamesOneRegistration(t *testing.T) {
	resetMemImages(t)
	src := UseImage("serial/a", 1, 1, pixels(1, 1, 1))
	first := serialOf(t, src)

	UseImage("serial/a", 1, 1, pixels(1, 1, 2)) // same size: no-op
	if got := serialOf(t, src); got != first {
		t.Errorf("same-size repeat changed the serial: %d -> %d", first, got)
	}
	// A registered key skips the decode and is only pinned, so the data
	// is never read here.
	if got, err := RegisterEncodedImage("serial/a", nil); err != nil || got != src {
		t.Fatalf("RegisterEncodedImage on a registered key = %q, %v", got, err)
	}
	if got := serialOf(t, src); got != first {
		t.Errorf("pinning changed the serial: %d -> %d", first, got)
	}

	UseImage("serial/a", 2, 2, pixels(2, 2, 3)) // new size: new registration
	resized := serialOf(t, src)
	if resized == first {
		t.Error("re-registration at a new size kept the serial")
	}

	DropImage("serial/a")
	UseImage("serial/a", 2, 2, pixels(2, 2, 4)) // same key, same size, after a drop
	if again := serialOf(t, src); again == resized || again == first {
		t.Errorf("drop + register reused a serial: %d (earlier %d, %d)", again, first, resized)
	}

	if s, ok := ImageSerial(src); !ok || s != serialOf(t, src) {
		t.Errorf("ImageSerial = %d, %t; want LookupImage's serial", s, ok)
	}
	for _, missing := range []string{"mem:serial/none", "serial/a", "/tmp/a.png", ""} {
		if s, ok := ImageSerial(missing); ok {
			t.Errorf("ImageSerial(%q) = %d, ok; want not ok", missing, s)
		}
	}
}

// resetMemImages must not restart the serials: a backend texture made
// before the reset would otherwise match a registration made after it.
func TestImageSerialSurvivesReset(t *testing.T) {
	resetMemImages(t)
	before := serialOf(t, UseImage("serial/reset", 1, 1, pixels(1, 1, 1)))
	resetMemImages(t)
	if after := serialOf(t, UseImage("serial/reset", 1, 1, pixels(1, 1, 1))); after == before {
		t.Errorf("serial %d reused across a reset", after)
	}
}

// The generation moves on every removal and only then, so a backend skips
// its texture check on every frame in which nothing left the registry.
func TestImageRegistryGenMovesOnRemovalOnly(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)
	const px = 16 // one 2x2 NRGBA buffer

	steps := []struct {
		name  string
		do    func()
		moves bool
	}{
		{"register a new key", func() { UseImage("gen/a", 2, 2, pixels(2, 2, 1)) }, false},
		{"repeat at the same size", func() { UseImage("gen/a", 2, 2, pixels(2, 2, 2)) }, false},
		{"pin an existing key", func() { pinImage("gen/a") }, false},
		{"look up and probe", func() {
			LookupImage("mem:gen/a")
			HasImage("gen/a")
			ImageSerial("mem:gen/a")
		}, false},
		{"drop a missing key", func() { DropImage("gen/none") }, false},
		{"re-register at a new size", func() { UseImage("gen/a", 1, 1, pixels(1, 1, 3)) }, true},
		{"drop", func() { DropImage("gen/a") }, true},
		{"budget eviction", func() {
			SetMemImageBudget(px)
			UseImage("gen/b", 2, 2, pixels(2, 2, 4))
			UseImage("gen/c", 2, 2, pixels(2, 2, 5)) // evicts gen/b
		}, true},
	}
	for _, s := range steps {
		before := ImageRegistryGen()
		s.do()
		if moved := ImageRegistryGen() != before; moved != s.moves {
			t.Errorf("%s: gen moved = %t, want %t", s.name, moved, s.moves)
		}
	}
	if HasImage("gen/b") {
		t.Fatal("gen/b survived; the eviction step did not evict")
	}
}

// ImageSerial is the backends' sweep probe: it must not refresh recency,
// or checking a cached texture would keep its image from being evicted.
func TestImageSerialLeavesRecencyAlone(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)
	const px = 16
	SetMemImageBudget(2 * px)
	UseImage("lru/a", 2, 2, pixels(2, 2, 1))
	UseImage("lru/b", 2, 2, pixels(2, 2, 2))
	if _, ok := ImageSerial("mem:lru/a"); !ok {
		t.Fatal("lru/a not registered")
	}
	UseImage("lru/c", 2, 2, pixels(2, 2, 3))
	if HasImage("lru/a") {
		t.Error("ImageSerial refreshed lru/a; want it evicted as least recently used")
	}
}

// IsMemImage is the backends' routing test, so it must agree with the
// prefix check the registry itself uses, registered key or not.
func TestIsMemImageExported(t *testing.T) {
	for _, src := range []string{"mem:a", "mem:", "mem", "a.png", "data:mem:", ""} {
		if IsMemImage(src) != isMemImage(src) {
			t.Errorf("IsMemImage(%q) disagrees with isMemImage", src)
		}
	}
}

// fakeTextureCache mirrors what a backend does with the registry: it keeps
// the serial each texture was uploaded from, sweeps when the generation
// moves, and re-uploads when a draw sees a different serial.
type fakeTextureCache struct {
	serial map[string]uint64 // Src -> serial of the "uploaded" pixels
	gen    uint64
}

func (c *fakeTextureCache) sweep() {
	gen := ImageRegistryGen()
	if gen == c.gen {
		return
	}
	c.gen = gen // before the walk, as the backends do
	for src, serial := range c.serial {
		if now, ok := ImageSerial(src); !ok || now != serial {
			delete(c.serial, src)
		}
	}
}

func (c *fakeTextureCache) draw(src string) (MemImage, bool) {
	mi, ok := LookupImage(src)
	if !ok {
		return MemImage{}, false
	}
	if c.serial[src] != mi.Serial {
		c.serial[src] = mi.Serial
	}
	return mi, true
}

// Concurrent registration, drop and eviction against a renderer that
// sweeps and draws. Run under -race (make prepush does) this pins the
// registry's locking; on any run it pins the invariants a backend relies
// on: a serial always comes with the same pixels and dimensions, and once
// registration stops one sweep leaves only textures that match the
// registry.
//
// Both sides yield every round. On js/wasm goroutines share one thread
// and a loop that never blocks is never preempted, so a reader that only
// polled would starve the writers and hang; with async preemption off the
// writers could finish before the reader ran at all.
func TestImageSerialConcurrentSweep(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)
	SetMemImageBudget(3 * 16) // small, so registration also evicts

	keys := []string{"race/0", "race/1", "race/2", "race/3"}
	srcs := make([]string, len(keys))
	for i, k := range keys {
		srcs[i] = memImagePrefix + k
	}
	memImages.mu.Lock()
	firstSerial := memImages.lastSerial
	memImages.mu.Unlock()

	const writers, rounds = 4, 400
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for i := range rounds {
				key := keys[(w+i)%len(keys)]
				switch i % 3 {
				case 0:
					UseImage(key, 2, 2, pixels(2, 2, byte(w)))
				case 1:
					UseImage(key, 1, 1, pixels(1, 1, byte(i)))
				default:
					DropImage(key)
				}
				runtime.Gosched()
			}
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	cache := &fakeTextureCache{serial: map[string]uint64{}}
	type seen struct {
		pix  *byte
		w, h int
	}
	bySerial := map[uint64]seen{}
	check := func() {
		cache.sweep()
		for _, src := range srcs {
			mi, ok := cache.draw(src)
			if !ok {
				continue
			}
			got := seen{pix: &mi.Pix[0], w: mi.W, h: mi.H}
			if prev, dup := bySerial[mi.Serial]; dup && prev != got {
				t.Fatalf("serial %d came with two buffers: %+v and %+v", mi.Serial, prev, got)
			}
			bySerial[mi.Serial] = got
		}
	}
	for running := true; running; {
		select {
		case <-done:
			running = false
		default:
			check()
			runtime.Gosched()
		}
	}
	check()

	for src, serial := range cache.serial {
		now, ok := ImageSerial(src)
		if !ok || now != serial {
			t.Errorf("after the last sweep %s holds serial %d, registry has %d (ok=%t)",
				src, serial, now, ok)
		}
	}
	// Count what the writers registered, not what the reader happened to
	// see: how often the two interleave is up to the scheduler. One
	// writer alone registers each key at two sizes in turn, so any run
	// makes more registrations than there are keys.
	memImages.mu.Lock()
	made := memImages.lastSerial - firstSerial
	memImages.mu.Unlock()
	if made <= uint64(len(keys)) {
		t.Fatalf("the writers made %d registrations for %d keys; the test did not "+
			"exercise re-registration", made, len(keys))
	}
	t.Logf("%d registrations made, %d observed by the renderer", made, len(bySerial))
}
