# Kisetsu uses WebKitGTK 4.1 (libwebkit2gtk-4.1-dev). Wails reads the tag
# from wails.json; plain Go commands need it passed explicitly.
TAGS := webkit2_41

# The version of the next release; the only place it is written.
VERSION := 0.7.1

# What this build calls itself: $(VERSION) only when built from the tag
# v$(VERSION) without changes, otherwise a development version such as
# 0.1.0~dev.142.g4d105fd (scripts/build-version.sh). Computed once, so
# `make deb` packages the binary under the same name. The release workflow
# passes it explicitly.
ifndef BUILD_VERSION
BUILD_VERSION := $(shell scripts/build-version.sh $(VERSION))
endif
LDFLAGS := -X github.com/sentivanac/kisetsu/internal/appinfo.Version=$(BUILD_VERSION)

# Application ID, used for the .desktop file and the icon name.
APP_ID := io.github.sentivanac.Kisetsu

.PHONY: dev build test test-live test-frontend vet check check-windows check-node icons deb windows version build-version frontend-dist

## dev: run the app with hot reload
dev: check-node
	wails dev

## build: produce a production binary in build/bin/
# The release binary is position independent, so ASLR covers it fully.
build: check-node
	GOFLAGS=-buildmode=pie wails build -ldflags "$(LDFLAGS)"

## deb: build a .deb package into build/deb/ (does not install it)
deb: build
	scripts/build-deb.sh $(BUILD_VERSION) $(APP_ID)

## windows: build kisetsu.exe and its installer into build/bin/ (needs
## nsis). The file version is VERSION (Windows wants numbers only); the
## installer's name carries BUILD_VERSION.
windows: check-node
	wails build -platform windows/amd64 -ldflags "$(LDFLAGS)"
	scripts/third-party-licenses.sh build/bin/kisetsu.exe > build/bin/THIRD_PARTY_LICENSES.txt
	@# Microsoft's WebView2 bootstrapper, as shipped in the Wails module;
	@# the installer runs it only when WebView2 is missing.
	mkdir -p build/windows/installer/tmp
	cp "$$(go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v2)/internal/webview2runtime/MicrosoftEdgeWebview2Setup.exe" \
		build/windows/installer/tmp/
	chmod 644 build/windows/installer/tmp/MicrosoftEdgeWebview2Setup.exe
	makensis -V2 -DARG_WAILS_AMD64_BINARY=../../bin/kisetsu.exe \
		-DINFO_PRODUCTVERSION=$(VERSION) -DKISETSU_VERSION=$(BUILD_VERSION) \
		build/windows/installer/project.nsi

## icons: render the PNG and Windows icons from the SVG source (needs
## inkscape and ImageMagick; the
## results are committed, so this is only needed after editing the SVG)
icons:
	for s in 16 24 32 48 64 128 256 512; do \
		inkscape build/linux/$(APP_ID).svg -o build/linux/icons/$$s.png -w $$s -h $$s; \
	done
	inkscape build/linux/$(APP_ID).svg -o build/appicon.png -w 1024 -h 1024
	convert $(foreach s,16 24 32 48 64 128 256,build/linux/icons/$(s).png) build/windows/icon.ico
	cp build/linux/icons/128.png frontend/src/assets/icon.png

## version: print the version of the next release
version:
	@echo $(VERSION)

## build-version: print what a build made now would call itself
build-version:
	@echo $(BUILD_VERSION)

# The Go code embeds frontend/dist (//go:embed), which is not in Git and
# which Vite empties on every build, so a fresh clone has none. Go tests
# do not need the real frontend; an empty placeholder lets them compile.
frontend-dist:
	@[ -n "$$(ls -A frontend/dist 2>/dev/null)" ] || { mkdir -p frontend/dist && touch frontend/dist/.placeholder; }

## test: run Go tests
test: frontend-dist
	go test -tags $(TAGS) ./...

## test-live: frontend-dist run tests against real provider APIs (uses the network)
test-live:
	go test -tags $(TAGS),live -count=1 -v -run Live ./internal/providers/... ./internal/search/ ./internal/downloads/

## test-frontend: run frontend unit tests (Vitest)
test-frontend: check-node
	cd frontend && npm test

## vet: run go vet
vet: frontend-dist
	go vet -tags $(TAGS) ./...

## check-windows: type-check the code and its tests for Windows (no
## Windows needed; the tests run on Windows in CI)
check-windows: frontend-dist
	GOOS=windows go vet ./...

## check: vet, Go tests, frontend type check and tests
check: vet check-windows test check-node
	cd frontend && npx tsc --noEmit && npm test

check-node:
	@node -e 'const v=process.versions.node; if (+v.split(".")[0] < 24) { console.error("Node " + v + " is too old; run: nvm use"); process.exit(1) }'
