NAME = grafsy
# This is space separated words, e.g. 'git@github.com leoleovich grafsy.git' or 'https   github.com leoleovich grafsy.git'
REPO_LIST = $(shell git remote get-url origin | tr ':/' ' ' )
# And here we take the word before the last to get the organization name
ORG_NAME = $(word $(words $(REPO_LIST)), first_word $(REPO_LIST))
# version in format $(tag without leading v).c$(commits since release).g$(sha1)
VERSION = $(shell git describe --long --tags 2>/dev/null | sed 's/^v//;s/\([^-]*-g\)/c\1/;s/-/./g')
VENDOR = "GitHub Actions of $(ORG_NAME)/$(NAME) <null@null.null>"
URL = https://github.com/$(ORG_NAME)/$(NAME)
define DESC =
'A very light proxy for graphite metrics with additional features
 This software receives carbon metrics localy, buffering them, aggregating, filtering bad metrics, and periodicaly sends them to one or few carbon servers'
endef
GO_FILES = $(shell find -name '*.go')
PKG_FILES = build/$(NAME)_$(VERSION)_amd64.deb build/$(NAME)-$(VERSION)-1.x86_64.rpm
SUM_FILES = build/sha256sum build/md5sum
GO_FLAGS =
GO_BUILD = go build $(GO_FLAGS) -ldflags "-X 'main.version=$(VERSION)'" -o $@ $<


.PHONY: all clean docker test version

all: build

version:
	@echo $(VERSION)

clean:
	rm -rf artifact
	rm -rf build

rebuild: clean all

# Run tests
test:
	go vet ./...
	go test -v ./...

build: build/$(NAME)

build/$(NAME): $(NAME)/main.go
	$(GO_BUILD)
