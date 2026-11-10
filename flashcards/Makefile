# Flashcards linux dev build with optional Android APK bundle.
#
# Targets:
#   dev            watch CSS (tailwind) + Go (air) with hot reload
#   desktop        build the desktop (linux) binary
#   css            compile tailwind classes once
#   images         copy the local images/ dir into assets/
#   run            build and run the desktop binary
#   apk            build the Android APK (requires an installed Android SDK/NDK/JDK)
#   keystore       generate a debug keystore for android release builds
#   apk-keystore   build the release APK with the debug keystore (test-only signing)
#   test           run go tests
#   clean          remove build artifacts
#
# Credits: build/android/scripts/go-dev-setup.sh sets up any missing deps.

GO ?= go
DESIGN_TOKENS ?= $(wildcard design_tokens.css)
ifeq ($(strip $(DESIGN_TOKENS)),)
	DESIGN_TOKENS_FLAG :=
else
	DESIGN_TOKENS_FLAG := --design-tokens $(DESIGN_TOKENS)
endif

.PHONY: all desktop dev watch-css watch-go css run apk keystore apk-keystore test clean

all: desktop

dev:
	@$(MAKE) -j2 watch-css watch-go

watch-css:
	tailwindcss -i assets/css/input.css -o assets/css/index.css --watch

watch-go:
	air

css:
	tailwindcss -i assets/css/input.css -o assets/css/index.css

desktop:
	$(GO) build -o flashcards .

run: desktop
	./flashcards

images:
	git ls-files images | tar -cf - -T - | tar -xf - -C . 2>/dev/null || true
	mkdir -p assets/images && cp -r images/. assets/images/ || true

test:
	$(GO) test ./...

clean:
	rm -f flashcards
	rm -rf assets/images

keystore:
	keytool -genkey -v -keystore build/android/debug.keystore \
		-alias flashcards -keyalg RSA -keysize 2048 -validity 100000 \
		-storepass android -keypass android \
		-dname "CN=Flashcards,OU=Dev,O=Personal,L=Berlin,S=Berlin,C=DE"

# Android needs the same embed assets plus a debug signing config.
apk: images
	cd build/android && ./gradlew assembleDebug -PflashcardsRoot=$(PWD) \
		-PflashcardsAssetsPath=$(PWD)/assets \
		$(DESIGN_TOKENS_FLAG)

apk-keystore:
	test -f build/android/debug.keystore || $(MAKE) keystore
	cd build/android && ./gradlew assembleRelease -PflashcardsRoot=$(PWD) \
		-PflashcardsAssetsPath=$(PWD)/assets \
		-PflashcardsKeystore=build/android/debug.keystore \
		-PflashcardsKeystorePassword=android \
		-PflashcardsKeyAlias=flashcards \
		$(DESIGN_TOKENS_FLAG)
