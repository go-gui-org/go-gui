SDK_PATH := $(shell xcrun --sdk iphonesimulator --show-sdk-path)
CC_PATH  := $(shell xcrun --sdk iphonesimulator --find clang)

APP_PATH = $(shell find ~/Library/Developer/Xcode/DerivedData/IOSDemo-*/Build/Products/Debug-iphonesimulator -name IOSDemo.app -maxdepth 1 2>/dev/null | head -1)
SIM      = iPhone 17 Pro

.PHONY: build run clean

build:
	CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
	  CC="$(CC_PATH)" \
	  CGO_CFLAGS="-isysroot $(SDK_PATH) -mios-simulator-version-min=17.0 -fembed-bitcode" \
	  CGO_LDFLAGS="-isysroot $(SDK_PATH) -mios-simulator-version-min=17.0" \
	  go build -buildmode=c-archive -tags ios -o ios/libgoguiapp.a .
	xcodebuild -project ios/IOSDemo.xcodeproj \
	  -scheme IOSDemo \
	  -sdk iphonesimulator \
	  -destination 'platform=iOS Simulator,name=iPhone 17 Pro' \
	  build

run: build
	xcrun simctl boot "$(SIM)" 2>/dev/null || true
	# Xcode 27 replaced Simulator.app with DeviceHub.app.
	open -a Simulator 2>/dev/null || open -a DeviceHub
	xcrun simctl install booted "$(APP_PATH)"
	xcrun simctl launch booted com.example.IOSDemo

clean:
	rm -f ios/libgoguiapp.a ios/libgoguiapp.h
	xcodebuild -project ios/IOSDemo.xcodeproj \
	  -scheme IOSDemo clean 2>/dev/null || true
