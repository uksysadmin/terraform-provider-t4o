VERSION     ?= 0.1.0
REGISTRY    := registry.terraform.io/trilio-demo/t4o
OS          := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH        := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
PLUGIN_DIR  := $(HOME)/.terraform.d/plugins/$(REGISTRY)/$(VERSION)/$(OS)_$(ARCH)
BINARY      := terraform-provider-t4o

.PHONY: build install clean

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(PLUGIN_DIR)
	mv $(BINARY) $(PLUGIN_DIR)/
	@echo "Installed to $(PLUGIN_DIR)/$(BINARY)"
	@echo "You can now run terraform init in your config directory."

clean:
	rm -f $(BINARY)
