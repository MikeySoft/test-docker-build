# Contributing to Flotilla

🙌 Thanks for your interest in contributing! Flotilla is a community-driven Docker management platform. This guide explains how to get set up, propose changes, and help steward the project.

## Code of Conduct

We are committed to a welcoming and harassment-free experience for everyone. By participating, you agree to follow the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/) and to report unacceptable behavior via security@mikeysoft.com.

## Ways to Contribute

- 🐛 **Report bugs** using the bug report issue template.
- 💡 **Request features** or enhancements via the feature request template.
- 🔧 **Submit patches** for bugs or new features.
- 📖 **Improve documentation** across `README.md`, `docs/`, and inline code comments.
- ✅ **Review pull requests** and help us maintain project velocity.

## Development Workflow

1. **Fork** the repository and create a topic branch from `main`.
2. **Install prerequisites** (Go 1.21+, Node.js 20+, Docker 24+, PostgreSQL 15+).
3. **Run setup scripts** outlined in `docs/development.md`.
4. **Make your changes**, keeping commits focused and logical.
5. **Run tests and linters** (see `Makefile` targets and CI workflows).
6. **Update documentation** and reference related issues in your commit messages.
7. **Open a pull request** following the template; include a summary, testing evidence, and screenshots when appropriate.

## Commit & Branch Guidelines

- Follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, etc.).
- Keep branches short-lived; rebase against `main` before opening a PR.
- Reference issues in the PR description rather than commit messages when possible.
- Include relevant subsystems in the subject line (e.g., `feat(server): add stack events API`).

## Testing Expectations

- `make test` must pass (Go unit tests, frontend unit tests, integration suites when feasible).
- `make lint` must pass (Go linters, `eslint`, `prettier`, Dockerfile lint).
- Add or update tests covering new functionality or bug fixes.
- For changes affecting the release process or deployment, run `make verify-release` as documented.

## Documentation Standards

- Update `README.md` for user-facing changes.
- Add deep-dive content to `docs/` for architecture, operations, or runbooks.
- Keep diagrams and screenshots current; store assets in `assets/`.

## Release Contributions

For release engineering work:

- Follow the versioning strategy in `docs/versioning.md`.
- Update `CHANGELOG.md` during release branches.
- Ensure Docker images and binaries are published via the automated workflows.

## Questions?

Open a discussion in GitHub Discussions or reach out on the community Slack (link coming soon). We appreciate your time and effort—welcome aboard! 🚢

