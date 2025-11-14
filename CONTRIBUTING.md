# Contributing to Instrumentation Score Service

Thank you for your interest in contributing! We welcome contributions from the community.

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue with:
- A clear, descriptive title
- Steps to reproduce the issue
- Expected vs actual behavior
- Your environment (OS, Go version, etc.)
- Relevant logs or error messages

### Suggesting Features

We welcome feature suggestions! Please open an issue with:
- A clear description of the feature
- Use cases and benefits
- Any implementation ideas you have

### Code Contributions

1. **Fork the repository** and create a new branch from `main`
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following these guidelines:
   - Write clear, readable code
   - Add tests for new functionality
   - Update documentation as needed
   - Follow existing code style and conventions

3. **Test your changes**
   ```bash
   make test
   make test-coverage
   ```

4. **Commit your changes** with clear, descriptive commit messages
   ```bash
   git commit -m "Add feature: description of what you added"
   ```

5. **Push to your fork** and submit a pull request
   ```bash
   git push origin feature/your-feature-name
   ```

### Pull Request Guidelines

- Keep PRs focused on a single feature or fix
- Include tests for new functionality
- Update documentation (README, FRAMEWORK.md, etc.) if needed
- Ensure all tests pass
- Reference related issues in your PR description

### Development Setup

```bash
# Clone the repository
git clone https://github.com/instrumentation-score-service/instrumentation-score.git
cd instrumentation-score

# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run tests with coverage
make test-coverage
```

### Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting
- Run `golangci-lint` before submitting
- Write clear comments for exported functions and types

### Adding New Rules

To add custom rules to the framework:

1. Define the rule in `rules_config.yaml`
2. Document the rule in `rules/RULE-ID.md`
3. Add tests demonstrating the rule
4. Update FRAMEWORK.md if introducing new concepts

See [FRAMEWORK.md](FRAMEWORK.md) for detailed guidance.

### Documentation

- Keep documentation up to date with code changes
- Use clear, concise language
- Include examples where helpful
- Check for broken links

### Community

- Be respectful and inclusive
- Follow our [Code of Conduct](CODE_OF_CONDUCT.md)
- Help others in issues and discussions
- Share your use cases and experiences

## Questions?

Feel free to:
- Open an issue for questions
- Start a discussion in GitHub Discussions
- Check existing issues and discussions first

Thank you for contributing! 🎉

