.PHONY: help build test test-coverage clean deps analyze evaluate install-completion

# Default target
help:
	@echo "Instrumentation Score Service - Available Commands:"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build              - Build the binary"
	@echo "  make test               - Run all tests"
	@echo "  make test-coverage      - Run tests with coverage report"
	@echo "  make clean              - Clean build artifacts"
	@echo ""
	@echo "Setup:"
	@echo "  make install-completion - Install shell autocomplete (bash/zsh/fish)"
	@echo ""
	@echo "Usage:"
	@echo "  make analyze            - Collect metrics from Prometheus (requires url, optional login)"
	@echo "  make evaluate job=<file|dir>  - Evaluate job(s) with HTML report and costs"
	@echo ""
	@echo "Examples:"
	@echo "  # Analyze metrics from local Prometheus (no auth)"
	@echo "  export url='http://localhost:9090'"
	@echo "  make analyze"
	@echo ""
	@echo "  # Analyze metrics from authenticated Prometheus"
	@echo "  export login='user:api_key'"
	@echo "  export url='https://your-prometheus-instance.com/api/prom'"
	@echo "  make analyze"
	@echo ""
	@echo "  # Analyze with filters (pass as make variable)"
	@echo "  make analyze filters='cluster=~\"prod.*\",environment=\"production\"'"
	@echo ""
	@echo "  # With pipe in regex (use double quotes)"
	@echo "  make analyze filters=\"cluster=~\\\"prod-1-27-a1|prod-1-27-a1-eu-central-1\\\"\""
	@echo ""
	@echo "  # Or set as environment variable"
	@echo "  export filters='cluster=~\"prod.*\"'"
	@echo "  make analyze"
	@echo ""
	@echo "  # With custom retry count (default is 2)"
	@echo "  make analyze retry=3"
	@echo ""
	@echo "  # Evaluate jobs"
	@echo "  make evaluate job=reports/job_metrics_*/api-service.txt  # single job"
	@echo "  make evaluate job=reports/job_metrics_20251102_160000/   # all jobs"

# Build the binary
build:
	go build -o instrumentation-score-service .

# Run all tests
test:
	go test -v ./pkg/... ./internal/...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./pkg/... ./internal/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	rm -f instrumentation-score-service
	rm -f coverage.out coverage.html
	rm -f *.json
	go clean

# Install dependencies
deps:
	go mod download
	go mod tidy

# Install shell completion
install-completion: build
	@echo "Installing shell completion..."
	@SHELL_NAME=$$(basename $$SHELL); \
	if [ "$$SHELL_NAME" = "bash" ]; then \
		if [ -d "$$(brew --prefix 2>/dev/null)/etc/bash_completion.d" ]; then \
			./instrumentation-score-service completion bash > $$(brew --prefix)/etc/bash_completion.d/instrumentation-score-service; \
			echo "✅ Bash completion installed to $$(brew --prefix)/etc/bash_completion.d/"; \
			echo "   Restart your shell or run: source $$(brew --prefix)/etc/bash_completion.d/instrumentation-score-service"; \
		elif [ -d "/etc/bash_completion.d" ]; then \
			sudo ./instrumentation-score-service completion bash > /etc/bash_completion.d/instrumentation-score-service; \
			echo "✅ Bash completion installed to /etc/bash_completion.d/"; \
			echo "   Restart your shell or run: source /etc/bash_completion.d/instrumentation-score-service"; \
		else \
			echo "❌ Bash completion directory not found"; \
			echo "   Run manually: source <(./instrumentation-score-service completion bash)"; \
		fi; \
	elif [ "$$SHELL_NAME" = "zsh" ]; then \
		mkdir -p ~/.zsh/completion; \
		./instrumentation-score-service completion zsh > ~/.zsh/completion/_instrumentation-score-service; \
		if ! grep -q "fpath=(~/.zsh/completion" ~/.zshrc 2>/dev/null; then \
			echo 'fpath=(~/.zsh/completion $$fpath)' >> ~/.zshrc; \
			echo 'autoload -U compinit; compinit' >> ~/.zshrc; \
		fi; \
		echo "✅ Zsh completion installed to ~/.zsh/completion/"; \
		echo "   Restart your shell or run: source ~/.zshrc"; \
	elif [ "$$SHELL_NAME" = "fish" ]; then \
		mkdir -p ~/.config/fish/completions; \
		./instrumentation-score-service completion fish > ~/.config/fish/completions/instrumentation-score-service.fish; \
		echo "✅ Fish completion installed to ~/.config/fish/completions/"; \
		echo "   Restart your shell or run: source ~/.config/fish/completions/instrumentation-score-service.fish"; \
	else \
		echo "❌ Unsupported shell: $$SHELL_NAME"; \
		echo "   Supported shells: bash, zsh, fish"; \
		echo "   Run manually: ./instrumentation-score-service completion --help"; \
	fi

# Analyze Prometheus metrics
analyze: build
	@if [ -z "$$url" ]; then \
		echo "Error: Required environment variable missing"; \
		echo ""; \
		echo "Set Prometheus URL (required):"; \
		echo "  export url='http://localhost:9090'"; \
		echo ""; \
		echo "For authenticated Prometheus, also set:"; \
		echo "  export login='user:api_key'"; \
		echo ""; \
		echo "Optional: Add query filters"; \
		echo "  make analyze filters='cluster=~\"prod.*\",environment=\"production\"'"; \
		exit 1; \
	fi
	@RETRY_FLAG=""; \
	if [ -n "$(retry)" ]; then \
		RETRY_FLAG="--retry-failures-count $(retry)"; \
	fi; \
	if [ -n '$(filters)' ]; then \
		echo 'Using filters: $(filters)'; \
		./instrumentation-score-service analyze \
			--output-dir ./reports \
			--additional-query-filters "$(filters)" \
			$$RETRY_FLAG; \
	else \
		./instrumentation-score-service analyze \
			--output-dir ./reports \
			$$RETRY_FLAG; \
	fi

# Evaluate job(s) - auto-detects single file or directory
evaluate: build
	@if [ -z "$(job)" ]; then \
		echo "Error: job is required"; \
		echo "Usage:"; \
		echo "  make evaluate job=path/to/job.txt        # single job"; \
		echo "  make evaluate job=path/to/job_metrics/   # all jobs"; \
		exit 1; \
	fi
	@if [ -f "$(job)" ]; then \
		HTML_FILE=$$(basename $(job) .txt)-report.html; \
		./instrumentation-score-service evaluate \
			--job-file $(job) \
			--rules rules_config.yaml \
			--output html \
			--html-file $$HTML_FILE && \
		echo "Report saved: $$HTML_FILE" && \
		open $$HTML_FILE 2>/dev/null || echo "Open with: open $$HTML_FILE"; \
	elif [ -d "$(job)" ]; then \
		HTML_FILE=all-jobs-report.html; \
		JSON_FILE=all-jobs-report.json; \
		./instrumentation-score-service evaluate \
			--job-dir $(job) \
			--rules rules_config.yaml \
			--output json,html \
			--json-file $$JSON_FILE \
			--html-file $$HTML_FILE \
			--show-costs \
			--cost-unit-price 0.00615 && \
		echo "HTML report saved: $$HTML_FILE" && \
		echo "JSON report saved: $$JSON_FILE" && \
		open $$HTML_FILE 2>/dev/null || echo "Open with: open $$HTML_FILE"; \
	else \
		echo "Error: $(job) is neither a file nor a directory"; \
		exit 1; \
	fi
