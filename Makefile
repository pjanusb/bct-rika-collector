APP := rika-collector
DIST := dist
IMAGE := rika-collector-builder

.PHONY: all release clean test linux windows assets docker docker-build docker-release

all: release

release: clean test linux windows assets

clean:
	mkdir -p $(DIST)
	find $(DIST) -mindepth 1 -maxdepth 1 ! -name dlogparser -exec rm -rf {} +

test:
	go test -mod=readonly ./...

linux:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath -ldflags="-s -w -buildid=" -o $(DIST)/$(APP)-amd64 ./cmd/rika-collector

windows:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=readonly -trimpath -ldflags="-s -w -buildid=" -o $(DIST)/$(APP)-win11.exe ./cmd/rika-collector

assets:
	cp config.example $(DIST)/config.env
	if [ -f $(DIST)/dlogparser ]; then chmod +x $(DIST)/dlogparser; fi

docker: docker-build docker-release

docker-build:
	docker build --pull -t $(IMAGE) .

docker-release:
	mkdir -p $(DIST)
	docker run --rm --user "$$(id -u):$$(id -g)" -e HOME=/tmp -v "$(CURDIR)/$(DIST):/workspace/$(DIST):Z" $(IMAGE)
