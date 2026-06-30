# v-rpc-tap — the `v rpc-tap` domain (durable, scalable RPC-broker tap -> S3).
# HYBRID Go+M repo (proposal D2): ships the VSL RPC TAP M package (VSLRT* + KIDS)
# AND the Go host. M routines MUTATE a national routine (XWBPRS splice) — isolated
# here, not in the additive-only v-stdlib.
#
# Engine work goes through the m-driver-sdk -> m-ydb/m-iris stack ONLY (m test
# --docker); never raw docker exec/iris session (org engine-access rule).
BIN ?= v-rpc-tap

# --- M side (VSL RPC TAP routines) ---
# DOCKER: bare test engine (ydb: m-test-engine, iris: m-test-iris).
# MSTDLIB: m-stdlib path — STDASSERT for the test harness only.
M       ?= m
ENGINE  ?= ydb
DOCKER  ?= m-test-engine
MSTDLIB ?= ../m-stdlib
TESTS   := tests/VSLRTAPTST.m tests/VSLRTHTST.m tests/VSLRTRPTST.m tests/VSLRTLTST.m tests/VSLRTCTST.m
ENGINE_FLAGS := --engine $(ENGINE) --docker $(DOCKER)
MIN_COVER ?= 85

m-fmt:
	$(M) fmt src tests
m-fmt-check:
	$(M) fmt --check src tests
m-lint:
	$(M) lint src tests
m-test:
	$(M) test $(TESTS) $(ENGINE_FLAGS) --routines src --routines $(MSTDLIB)/src
m-coverage:
	$(M) coverage $(ENGINE_FLAGS) --routines $(MSTDLIB)/src --min-percent=$(MIN_COVER) src tests
# both engines, the dual-engine gate
m-test-dual:
	$(MAKE) m-test ENGINE=ydb  DOCKER=m-test-engine
	$(MAKE) m-test ENGINE=iris DOCKER=m-test-iris
# D10 dependency-purity gate: VSLRT* reference no STD*/external-VSL*, no crypto/json/net
m-purity:
	./scripts/purity-check.sh
# build the VSL RPC TAP KIDS transport global (VSLRT*1.0*1) from the declarative spec
kids:
	v pkg build kids/vslrtap.build.json --src src --out dist/kids/vslrtap.kids
m-check: m-fmt-check m-lint m-purity m-test-dual m-coverage

# --- Go side (host; empty until P3) ---
go-build:
	go build ./...
go-test:
	go test -race -cover ./...
go-lint:
	golangci-lint run ./...
go-check: go-lint go-test go-build

# --- combined ---
check: m-check go-check

clean:
	rm -f dist/$(BIN) dist/$(BIN)-* *.test
