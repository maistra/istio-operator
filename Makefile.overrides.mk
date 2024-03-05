.DEFAULT_GOAL := default
BUILD_WITH_CONTAINER ?= 1
CONTAINER_OPTIONS = --mount type=bind,source=/tmp,destination=/tmp --net=host

export COMMONFILES_POSTPROCESS = tools/commonfiles-postprocess.sh

ifeq ($(BUILD_WITH_CONTAINER),1)
# create phony targets for the top-level items in the repo
PHONYS := $(shell ls | grep -v Makefile)
.PHONY: $(PHONYS)
$(PHONYS):
	@$(MAKE_DOCKER) $@
endif

MAKEFILE_LIST = Makefile Makefile.core.mk Makefile.overrides.mk

# help works by looking over all Makefile includes matching `target: ## comment` regex and outputting them
.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[\.a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
