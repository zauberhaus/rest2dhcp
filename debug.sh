#!/bin/sh

sudo -E dlv debug --api-version=2 --headless --listen=:2345 --log -- $@
