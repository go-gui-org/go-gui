# Headless Wayland test image for the experimental Wayland backend (#919).
#
# It holds three compositors that can run with no GPU and no screen, so the
# backend can be built and run on a Mac (colima) or a CI runner:
#
#   sway   (wlroots)  screenshots with grim, input injected with wtype/wlrctl
#   weston (reference) screenshots with weston-screenshooter
#   mutter (GNOME)    no server-side decorations, so it tests the libdecor path
#
# Mesa's llvmpipe gives desktop OpenGL on the Wayland EGL platform. The
# Wayland client libraries are what the backend dlopens through purego:
# libwayland-client, libwayland-egl, libxkbcommon and libdecor.
#
# The run.sh next to this file builds and starts it; do not run it by hand.
FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates dbus dbus-user-session procps \
      sway grim wtype wlrctl \
      weston \
      mutter gnome-settings-daemon-common gsettings-desktop-schemas \
      libwayland-client0 libwayland-egl1 libwayland-cursor0 \
      libxkbcommon0 xkb-data \
      libdecor-0-0 libdecor-0-plugin-1-gtk \
      libegl1 libegl-mesa0 libgl1 libgl1-mesa-dri mesa-utils \
      fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*

# Compositors refuse to run as root, or behave differently when they do.
RUN useradd --create-home --uid 1000 tester
USER tester
ENV XDG_RUNTIME_DIR=/tmp/xdg LIBGL_ALWAYS_SOFTWARE=1 GALLIUM_DRIVER=llvmpipe
