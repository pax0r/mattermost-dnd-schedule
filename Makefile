.PHONY: all verify release version check-version dist deps test test-server test-webapp test-release typecheck clean

export VERSION

all: dist

verify: deps
	$(MAKE) test
	$(MAKE) dist
	python3 scripts/check_bundle.py dist/*.tar.gz

release: verify
	python3 scripts/release.py

version:
	python3 scripts/version.py

check-version:
	python3 scripts/version.py --check

deps:
	go mod download
	cd webapp && if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; else npm install --no-audit --no-fund; fi

test: check-version test-server test-webapp test-release typecheck

test-release:
	python3 -m unittest discover -s scripts -p 'test_*.py' -v
	node --test --experimental-test-isolation=none scripts/test_publish.cjs

test-server:
	go test -mod=readonly -race -count=1 ./server/...

test-webapp:
	cd webapp && npm test

typecheck:
	cd webapp && npm run typecheck

dist:
	cd webapp && npm run build
	python3 scripts/licenses.py
	python3 scripts/bundle.py

clean:
	rm -rf dist server/dist webapp/dist
