# Kisetsu uses WebKitGTK 4.1 (libwebkit2gtk-4.1-dev). Wails reads the tag
# from wails.json; plain Go commands need it passed explicitly.
TAGS := webkit2_41

# The version of the next release; the only place it is written.
VERSION := 0.1.0
LDFLAGS := -X github.com/sentivanac/kisetsu/internal/appinfo.Version=$(VERSION)

# Application ID, used for the .desktop file and the icon name.
APP_ID := io.github.sentivanac.Kisetsu

.PHONY: dev build test test-live test-frontend vet check check-node icons deb version frontend-dist

## dev: run the app with hot reload
dev: check-node
	wails dev

## build: produce a production binary in build/bin/
# The release binary is position independent, so ASLR covers it fully.
build: check-node
	GOFLAGS=-buildmode=pie wails build -ldflags "$(LDFLAGS)"

## deb: build a .deb package into build/deb/ (does not install it)
deb: build
	scripts/build-deb.sh $(VERSION) $(APP_ID)

## icons: render the PNG icons from the SVG source (needs inkscape; the
## results are committed, so this is only needed after editing the SVG)
icons:
	for s in 16 24 32 48 64 128 256 512; do \
		inkscape build/linux/$(APP_ID).svg -o build/linux/icons/$$s.png -w $$s -h $$s; \
	done
	inkscape build/linux/$(APP_ID).svg -o build/appicon.png -w 1024 -h 1024

## version: print the version of the next release
version:
	@echo $(VERSION)

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

## check: vet, Go tests, frontend type check and tests
check: vet test check-node
	cd frontend && npx tsc --noEmit && npm test

check-node:
	@node -e 'const v=process.versions.node; if (+v.split(".")[0] < 24) { console.error("Node " + v + " is too old; run: nvm use"); process.exit(1) }'
