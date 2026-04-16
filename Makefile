BINARY=fscan
BUILD_DIR=./build
DART_WORKER_DIR=./dart_worker

.PHONY: all build test clean dart-setup run-sample

all: build

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./...

lint:
	go vet ./...

dart-setup:
	cd $(DART_WORKER_DIR) && dart pub get

run-sample: build
	$(BUILD_DIR)/$(BINARY) scan testdata/sample_app --skip-dart --verbose

run-sample-full: build dart-setup
	$(BUILD_DIR)/$(BINARY) scan testdata/sample_app \
		--dart-worker $(DART_WORKER_DIR)/bin/worker.dart \
		--verbose

clean:
	rm -rf $(BUILD_DIR)

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)
