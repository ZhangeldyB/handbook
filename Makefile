# Verification for the handbook itself. Not installed into service repositories.
.PHONY: help verify verify-examples verify-lint verify-docs verify-hooks verify-install

help:
	@echo "Targets:"
	@echo "  verify          Run every check below"
	@echo "  verify-examples go vet + go test for every example module"
	@echo "  verify-lint     Validate templates/golangci.yml and lint examples/orders with it"
	@echo "  verify-docs     Links, anchors, .handbook/ references, banned terms, frontmatter, root budget"
	@echo "  verify-hooks    Hook syntax and PreToolUse/PostToolUse fixtures"
	@echo "  verify-install  Install into a scratch repository and check the projection"

verify: verify-examples verify-lint verify-docs verify-hooks verify-install

verify-examples:
	cd examples/orders && go vet ./... && go test -count=1 ./...
	cd examples/grpctransport && go vet ./... && go test -count=1 ./...
	cd examples/antipatterns && go vet ./... && go build ./...

verify-lint:
	golangci-lint config verify -c templates/golangci.yml
	cd examples/orders && golangci-lint run -c ../../templates/golangci.yml ./...

verify-docs:
	go run ./tools/doccheck -root .

verify-hooks:
	bash .agents/hooks/testdata/run-hook-tests.sh

verify-install:
	@tmp=$$(mktemp -d) && git -C "$$tmp" init -q && \
	bash scripts/install.sh "$$tmp" >/dev/null && \
	bash scripts/install.sh "$$tmp" >/dev/null && \
	cmp guides/ai-agent-instructions.md "$$tmp/AGENTS.md" && \
	cmp guides/ai-agent-instructions.md "$$tmp/CLAUDE.md" && \
	test "$$(jq '.hooks.Stop | length' "$$tmp/.claude/settings.json")" = 1 && \
	test -f "$$tmp/.handbook/patterns/outbox.md" && \
	test ! -e "$$tmp/.handbook/tools" && \
	rm -rf "$$tmp" && echo "install projection ok"
