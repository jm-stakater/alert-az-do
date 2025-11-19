DOCKER_REPO             ?= ghcr.io/alert-az-do
DOCKER_IMAGE_NAME       ?= alert-az-do


# "common-coverage" also runs tests, so common-test is not needed when running common-coverage
.PHONY: all # Similar to default command for common, but without yamllint
#all: precheck style check_license lint unused build test
all: precheck style check_license lint build coverage

include Makefile.common
