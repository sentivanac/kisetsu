# Kisetsu uses WebKitGTK 4.1 (libwebkit2gtk-4.1-dev). Wails reads the tag
# from wails.json; plain Go commands need it passed explicitly.
TAGS := webkit2_41

.PHONY: dev build test vet check check-node

## dev: run the app with hot reload
dev: check-node
	wails dev

## build: produce a production binary in build/bin/
build: check-node
	wails build

## test: run Go tests
test:
	go test -tags $(TAGS) ./...

## vet: run go vet
vet:
	go vet -tags $(TAGS) ./...

## check: vet + tests + frontend type check
check: vet test check-node
	cd frontend && npx tsc --noEmit

check-node:
	@node -e 'const v=process.versions.node; if (+v.split(".")[0] < 24) { console.error("Node " + v + " is too old; run: nvm use"); process.exit(1) }'
