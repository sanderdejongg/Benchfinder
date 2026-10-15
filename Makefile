# Command prefix for running tools in the app container (6-architecture-setup D28).
# Run from the repository root. CI overrides it with an empty value: make EXEC= ...
EXEC ?= docker compose --project-directory backend exec -T php

# Tool paths are relative to the backend root: the container workdir locally,
# `working-directory: backend` in CI (make -f ../Makefile EXEC= <target>).
.PHONY: check fix test

check:
	$(EXEC) vendor/bin/php-cs-fixer fix --dry-run --diff
	$(EXEC) vendor/bin/phpcs
	$(EXEC) vendor/bin/phpstan analyse
	$(EXEC) vendor/bin/rector process --dry-run

# phpcbf exits 1 when it fixed everything; only 2 and 3 are failures.
fix:
	$(EXEC) vendor/bin/rector process
	$(EXEC) vendor/bin/php-cs-fixer fix
	$(EXEC) vendor/bin/phpcbf || [ $$? -eq 1 ]

test:
	$(EXEC) vendor/bin/phpunit
