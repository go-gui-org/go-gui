# iOS Simulator Demo

Runs go-gui on the iOS Simulator. No Apple Developer account needed.

## Prerequisites

- macOS with Xcode installed (`xcode-select -p` to verify)
- At least one iOS simulator runtime (`xcrun simctl list runtimes`)

## Run

```bash
# Download and unzip the release artifact, then:

# 1. Boot a simulator (or use an already-booted one)
open -a Simulator

# 2. Install the app
xcrun simctl install booted IOSDemo.app

# 3. Launch it
xcrun simctl launch booted com.example.IOSDemo
```

To use a specific simulator instead of `booted`:

```bash
# List available devices
xcrun simctl list devices

# Boot a specific one
xcrun simctl boot "iPhone 17 Pro"

# Install and launch on that device
xcrun simctl install "iPhone 17 Pro" IOSDemo.app
xcrun simctl launch "iPhone 17 Pro" com.example.IOSDemo
```

## Troubleshooting

**"Unable to load simulator devices" or CoreSimulator version mismatch**

Your Xcode and macOS simulator runtime are out of sync. Update macOS
(System Settings → Software Update) or install a matching simulator
runtime in Xcode (Settings → Platforms).

**"The app couldn't be installed"**

Make sure the simulator is booted before installing.

**App launches to a black screen**

The app uses Metal for rendering. Simulators on Apple Silicon support
Metal; Intel Macs may have limited support.
