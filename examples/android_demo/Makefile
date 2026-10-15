ANDROID_HOME ?= $(HOME)/Library/Android/sdk
export ANDROID_HOME

.PHONY: bind build install clean

bind:
	gomobile bind -target=android/arm64,android/amd64 -androidapi 24 \
	  -o android/app/libs/gogui.aar ./

build: bind
	cd android && ./gradlew assembleDebug

install: build
	cd android && ./gradlew installDebug

clean:
	rm -f android/app/libs/gogui.aar android/app/libs/gogui-sources.jar
	cd android && ./gradlew clean 2>/dev/null || true
