DOCKER_REPO             ?= quay.io/alert-az-do
DOCKER_IMAGE_NAME       ?= alert-az-do


.PHONY: all # Similar to default command for common, but without yamllint
all: precheck style check_license lint unused build test

include Makefile.common
