#!/bin/bash

sudo go test -v -timeout 30s github.com/zauberhaus/rest2dhcp/service -run ^Test.*$
sudo go test -v -timeout 30s github.com/zauberhaus/rest2dhcp/client -run ^Test.*$
