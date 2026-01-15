.PHONY: build clean install

BINARY_NAME=amgctl
BUILD_DIR=build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

test:
	go test ./...
