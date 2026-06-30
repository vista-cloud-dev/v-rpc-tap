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
TESTS   := tests/VSLRTAPTST.m tests/VSLRTHTST.m tests/VSLRTRPTST.m tests/VSLRTLTST.m tests/VSLRTCTST.m tests/VSLRTLDTST.m
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
# Build the VSL RPC TAP KIDS transport global (VSLRT*1.0*1) from the declarative
# spec. The build overwrites the national XWBPRS, so its routine is GENERATED from
# the TARGET SITE's CURRENT XWBPRS (proposal §12 — never a frozen repo copy): pull
# it read-only over the driver (`m sync diff/pull`) and pass its path as SPLICE_SRC.
# The 3 VSLRT* routines come from src/; the spliced XWBPRS is staged alongside them.
# v pkg ships/installs/restores the result (the audited foreign-overwrite path);
# nothing here touches a live engine.
SPLICE_SRC ?=
KIDS_SRC = dist/build-src

.PHONY: kids   # the `kids` recipe must always run (the kids/ directory would otherwise satisfy it)

dist/rpctap: cmd/rpctap/main.go $(wildcard rpctapcli/*.go) $(wildcard internal/splice/*.go)
	go build -o dist/rpctap ./cmd/rpctap

kids: dist/rpctap
	@test -n "$(SPLICE_SRC)" || { echo "kids: set SPLICE_SRC=<the target site's current XWBPRS.m> — pull it read-only via 'm sync diff/pull' over the driver; the splice is regenerated per-site, never a frozen copy"; exit 2; }
	mkdir -p $(KIDS_SRC) dist/kids
	cp src/VSLRTAP.m src/VSLRTH.m src/VSLRTRP.m $(KIDS_SRC)/
	dist/rpctap splice --in "$(SPLICE_SRC)" --out $(KIDS_SRC)/XWBPRS.m
	v pkg build kids/vslrtap.build.json --src $(KIDS_SRC) --out dist/kids/vslrtap.kids
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
