APP := rika-collector
DIST := dist
IMAGE := rika-collector-builder
VERSION := $(shell tr -d '\r\n' < rika-collector-version.txt)
LDFLAGS := -s -w -buildid= -X main.version=$(VERSION)

.PHONY: all release build clean test linux windows assets docker-build docker-release

all: release

build: clean test linux windows assets

clean:
	mkdir -p $(DIST)
	find $(DIST) -mindepth 1 -maxdepth 1 ! -name dlogparser -exec rm -rf {} +

test:
	go test -mod=readonly ./...

linux:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath -ldflags="$(LDFLAGS)" -o $(DIST)/$(APP)-amd64 ./cmd/rika-collector

windows:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=readonly -trimpath -ldflags="$(LDFLAGS)" -o $(DIST)/$(APP)-win11.exe ./cmd/rika-collector

assets:
	cp config.example $(DIST)/collector.conf
	cp rika-collector-win11.bat $(DIST)/rika-collector-win11.bat
	if [ -f $(DIST)/dlogparser ]; then chmod +x $(DIST)/dlogparser; fi

release: docker-build docker-release

docker-build:
	docker build --pull -t $(IMAGE) .

docker-release:
	mkdir -p $(DIST)
	docker run --rm --user "$$(id -u):$$(id -g)" -e HOME=/tmp -v "$(CURDIR)/$(DIST):/workspace/$(DIST):Z" $(IMAGE)
