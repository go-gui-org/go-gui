# Handoff: #824 GL backend heap-corruption crash

Branch-only notes. Delete this file before the branch goes to a PR.

Status on 2026-09-28: the crash in #824 is not proven to be a go-gui bug. Every
reproduction so far came from an emulated amd64 container, and that emulator
breaks plain Go programs too. No code change has been made.

## What was found

1. The Docker "linux/amd64" container on the arm64 Mac runs under **qemu-user**:
   inside it, `/proc/1/exe -> /usr/bin/qemu-x86_64`.
2. A plain static Go program (no purego, no C, no GL; the source is below) fails
   **10 of 10** there with `fatal error: concurrent map writes`. Built for
   linux/arm64 and run natively in the `gogui-gl` image, it fails **0 of 10**.
   Go 1.26.8 and 1.27.1 both fail.
3. The backend crash rate did not change with Go 1.26 vs 1.27, purego v0.10.2 vs
   v0.11.1, `GOEXPERIMENT=nogreenteagc`, `norandomizedheapbase64`,
   `asyncpreemptoff=1` or the Mesa thread settings (`LP_NUM_THREADS=0`,
   `MESA_SHADER_CACHE_DISABLE`).
4. With `GODEBUG=gcstoptheworld=2` the test binary crashes in **package init**
   (regexp compile), before any EGL or GL call.
5. `GOMAXPROCS=1` lowered the rate (0 of 20). This fits the likely cause: on an
   ARM host, qemu-user does not keep x86's strict memory ordering, and the Go
   runtime depends on that ordering on amd64.
6. The locked thread that makes the EGL calls is a normal glibc pthread with an
   8 MB stack. This rules out C code running on a Go-allocated stack or TLS.

## Open items

1. **Real amd64.** CI job "GL render (Mesa)" (`.github/workflows/ci.yml`, runs
   on ubuntu-latest, real amd64) runs `scripts/gl-render-test.sh`, which retries
   a crashed test and prints `crashed without a verdict`. Count that warning in
   recent runs (fish):

   ```fish
   for id in (gh run list --workflow ci.yml --limit 15 --json databaseId --jq '.[].databaseId')
       echo $id (gh run view $id --log 2>/dev/null | grep -c "crashed without a verdict")
   end
   ```

   Or run the loop script below on a real Linux amd64 box with Xvfb + Mesa. If
   the count is zero: close the heap part of #824 (only after a merged change,
   per the working agreements) and remove the retry loop from
   `scripts/gl-render-test.sh`.

2. **Native linux/arm64 segfault (exit 139)** after the first backend. This was
   seen natively, not emulated, so it may be real. Test it in the `gogui-gl`
   image (arm64, no emulation) with an arm64 build of the test binary.

3. **A second backend in one process gets its X connection reset.** Not checked
   on native hardware yet.

## Reproduction kit

Build the test binary (from the repo root, macOS):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o ~/.cache/gl824/gl.test ./gui/backend/gl/
```

Use `GOARCH=arm64` for item 2. Mount the directory under `$HOME` (Docker did not
see files under `/private/tmp`).

`~/.cache/gl824/loop.sh` runs one test N times, each in its own process, and
keeps a log per crash:

```bash
#!/bin/bash
# Usage: loop.sh <binary> <test> <n>
bin=$1; t=$2; n=$3; tag=$(basename "$bin" .test)
d=$((100 + RANDOM % 800))
Xvfb :$d -screen 0 1024x768x24 >/dev/null 2>&1 &
sleep 1
export DISPLAY=:$d GOGUI_REQUIRE_GL=1 GOTRACEBACK=crash
mkdir -p /w/logs
crash=0
for i in $(seq 1 "$n"); do
  out=$(timeout 60 "$bin" -test.run "^$t\$" -test.v 2>&1)
  if ! printf '%s' "$out" | grep -q -- "--- PASS: $t"; then
    crash=$((crash+1)); printf '%s\n' "$out" > "/w/logs/$tag-$i.log"
  fi
done
echo "$tag $t: $crash of $n crashed"
```

Run it:

```sh
docker run --rm --platform linux/amd64 -v ~/.cache/gl824:/w gogui-gl-amd64 \
  /w/loop.sh /w/gl.test TestBackendRenderSmoke 20
```

On a real Linux box, run `loop.sh` directly with `/w` replaced by a local
directory.

Emulator check: the plain Go program. Build it with
`GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`. It prints `ok` on a
healthy platform:

```go
package main

import (
	"fmt"
	"regexp"
	"runtime"
)

func main() {
	type node struct {
		next *node
		v    [6]int
		m    map[int]string
	}
	var keep []*node
	for round := 0; round < 200; round++ {
		var head *node
		for i := 0; i < 2000; i++ {
			n := &node{next: head, m: map[int]string{i: fmt.Sprint(i)}}
			n.v[0] = i
			head = n
		}
		keep = append(keep, head)
		if len(keep) > 20 {
			keep = keep[1:]
		}
		if round%10 == 0 {
			runtime.GC()
		}
		for _, h := range keep {
			i := 1999
			for n := h; n != nil; n = n.next {
				if n.v[0] != i || n.m[i] != fmt.Sprint(i) {
					panic(fmt.Sprintf("corrupt at round %d", round))
				}
				i--
			}
		}
	}
	regexp.MustCompile(`^(?:http|https|ftp)://[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+`)
	fmt.Println("ok")
}
```

Useful trick: forcing `runtime.GC()` twice after each step of `gl.New` turned
the roughly 20% crash rate into roughly 100%. That makes bisecting fast if a
real crash turns up on native hardware.
