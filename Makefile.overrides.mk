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
