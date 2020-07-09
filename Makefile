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

docker:
	docker build -t $(ORG_NAME)/$(NAME):builder -f docker/builder/Dockerfile .
	docker build --build-arg IMAGE=$(ORG_NAME)/$(NAME) -t $(ORG_NAME)/$(NAME):latest -f docker/$(NAME)/Dockerfile .

build/$(NAME): $(NAME)/main.go
	$(GO_BUILD)

#########################################################
# Prepare artifact directory and set outputs for upload #
#########################################################
github_artifact: $(foreach art,$(PKG_FILES) $(SUM_FILES), artifact/$(notdir $(art)))

artifact:
	mkdir $@

# Link artifact to directory with setting step output to filename
artifact/%: ART=$(notdir $@)
artifact/%: TYPE=$(lastword $(subst ., ,$(ART)))
artifact/%: build/% | artifact
	cp -l $< $@
	@echo '::set-output name=$(TYPE)::$(ART)'

#######
# END #
#######

#############
# Packaging #
#############

# Prepare everything for packaging
.ONESHELL:
build/pkg: build/$(NAME)_linux_x64 $(NAME).toml
	cd build
	mkdir -p pkg/etc/$(NAME)/example/
	mkdir -p pkg/usr/bin
	cp -l $(NAME)_linux_x64 pkg/usr/bin/$(NAME)
	cp -l ../$(NAME).toml pkg/etc/$(NAME)/example/

build/$(NAME)_linux_x64: $(NAME)/main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD)

packages: $(PKG_FILES) $(SUM_FILES)

# md5 and sha256 sum-files for packages
$(SUM_FILES): COMMAND = $(notdir $@)
$(SUM_FILES): PKG_FILES_NAME = $(notdir $(PKG_FILES))
.ONESHELL:
$(SUM_FILES): $(PKG_FILES)
	cd build
	$(COMMAND) $(PKG_FILES_NAME) > $(COMMAND)

deb: $(word 1, $(PKG_FILES))

rpm: $(word 2, $(PKG_FILES))

# Set TYPE to package suffix w/o dot
$(PKG_FILES): TYPE = $(subst .,,$(suffix $@))
$(PKG_FILES): build/pkg
	fpm --verbose \
		-s dir \
		-a x86_64 \
		-t $(TYPE) \
		--vendor $(VENDOR) \
		-m $(VENDOR) \
		--url $(URL) \
		--description $(DESC) \
		--license Apache \
		-n $(NAME) \
		-v $(VERSION) \
		--after-install packaging/postinst \
		--before-remove packaging/prerm \
		-p build \
		build/pkg/=/ \
		packaging/$(NAME).service=/lib/systemd/system/$(NAME).service

#######
# END #
#######
