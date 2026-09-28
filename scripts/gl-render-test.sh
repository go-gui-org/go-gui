#!/usr/bin/env bash
# Runs the GL backend tests that need a real GL 3.3 context (#823).
#
# `go test ./gui/...` skips them, because a CI runner has no GPU and no
# display. The CI jobs that call this script provide Mesa's software
# renderer (llvmpipe): on Linux under Xvfb, on Windows by putting Mesa's
# opengl32.dll next to the test binary. GOGUI_REQUIRE_GL makes a test fail,
# instead of skip, when the backend still cannot open a context, so a broken
# runner setup shows up as red and not as a silent pass.
#
# Usage: scripts/gl-render-test.sh <dir for the test binary>
# The caller copies opengl32.dll into that directory first on Windows: the
# app directory is the only place Windows looks before System32, whose
# opengl32.dll is GDI's OpenGL 1.1.
set -uo pipefail

dir=${1:?usage: gl-render-test.sh <dir>}
bin="$dir/gl.test"
[ "${OS:-}" = Windows_NT ] && bin="$dir/gl.test.exe"

CGO_ENABLED=0 go test -c -o "$bin" ./gui/backend/gl/ || exit 1

# One process per test. Under Xvfb a second backend in the same process gets
# its X connection reset (cause not traced). On linux/arm64 Mesa the process
# also segfaults shortly after its first backend, on main as well, so run
# this on linux/amd64, as CI does.
#
# The "found bad pointer in Go heap" crashes tracked as #824 were reproduced
# only under qemu-user emulation (an amd64 container on an arm64 host), where
# a plain Go program with no GL, no C and no purego corrupts its own heap too.
# On real linux/amd64 hardware the same binary ran 270 single-test processes
# and 10 whole-package processes clean, on the native driver and on llvmpipe.
# So there is no retry here: a run that crashes before reporting a verdict is
# a failure, like any other.
status=0
for t in TestTriangleEdgesAntialiased TestProbeSeesAliasedEdges \
  TestFilterEdgesAntialiased TestFilterProbeSeesAliasedEdges TestRefusedResolveFallsBack \
  TestBackendRenderSmoke; do
  out=$(GOGUI_REQUIRE_GL=1 "$bin" -test.run "^$t\$" -test.v 2>&1)
  code=$?
  printf '%s\n' "$out"
  if printf '%s\n' "$out" | grep -q -- "--- FAIL: $t"; then
    result=fail
  elif printf '%s\n' "$out" | grep -q -- "--- SKIP: $t"; then
    result=skip
  elif [ "$code" -eq 0 ] && printf '%s\n' "$out" | grep -q -- "--- PASS: $t"; then
    result=pass
  else
    result=crash
  fi
  case $result in
  pass) ;;
  fail)
    echo "::error::$t failed"
    status=1
    ;;
  skip)
    # A skipped test checked nothing: the runner has no usable GL context.
    echo "::error::$t skipped: no usable GL context on this runner"
    status=1
    ;;
  crash)
    echo "::error::$t crashed without reporting a verdict (exit $code)"
    status=1
    ;;
  esac
done
exit "$status"
