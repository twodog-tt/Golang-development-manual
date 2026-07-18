SHELL := /usr/bin/env bash
CHAIN ?=
LANE ?= n

.PHONY: manifests upstream-tags offline build toxiproxy-up toxiproxy-down start stop gate faults restart-gate upgrade-gate clean

manifests:
	./scripts/check-manifests.sh

upstream-tags:
	LOCALNET_ALLOW_NETWORK=1 ./scripts/check-upstream-tags.sh

offline:
	./scripts/test-offline.sh $(CHAIN)

build:
	LOCALNET_ALLOW_NETWORK=1 ./scripts/build-node.sh $(CHAIN) $(LANE)

toxiproxy-up:
	./scripts/toxiproxy.sh up

toxiproxy-down:
	./scripts/toxiproxy.sh down

start:
	./scripts/node.sh start $(CHAIN) $(LANE) fresh

stop:
	./scripts/node.sh stop $(CHAIN)

gate:
	./scripts/gate.sh $(CHAIN) baseline proxy

faults:
	./scripts/fault-gate.sh $(CHAIN)

restart-gate:
	./scripts/restart-gate.sh $(CHAIN)

upgrade-gate:
	./scripts/upgrade-gate.sh $(CHAIN)

clean:
	./scripts/node.sh stop $(CHAIN)
	rm -rf .state
