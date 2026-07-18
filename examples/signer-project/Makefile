.PHONY: test test-race demo frost-demo mpc-test compose-config softhsm hsm-acceptance etcd-compose-test etcd-local-test

test:
	go test ./...

test-race:
	go test -race ./...

demo:
	go run ./cmd/signer-demo

frost-demo:
	go run ./cmd/frost-demo

mpc-test:
	go test -count=1 -race ./backend/frostcluster

compose-config:
	docker compose config --quiet

softhsm:
	docker compose run --rm softhsm-signer

hsm-acceptance:
	go run ./cmd/hsm-acceptance

etcd-compose-test:
	docker compose -f compose.etcd.yaml up -d --wait
	ETCD_ENDPOINTS=http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 \
	ETCD_REQUIRE_THREE_MEMBERS=1 \
	go test -count=1 -tags=etcd_integration -run '^TestEtcd' ./fence

etcd-local-test:
	./scripts/run-etcd-integration.sh
