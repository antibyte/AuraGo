# Contributing to AuraGo

First off, thank you for your interest in contributing to AuraGo! It's because of people like you that AuraGo is able to grow and serve the self-hosted and homelab community.

Please take a moment to review this guide before making your first contribution. It will help make your contribution process smooth, efficient, and successful!

---

## 🗺️ Code of Conduct

By participating in this project, you agree to abide by our **[Code of Conduct](CODE_OF_CONDUCT.md)**. Please report any unacceptable behavior to **maintainers@aurago.dev**.

---

## 🛠️ Setting Up Your Development Environment

AuraGo is written in Go, and the frontend is an embedded Web UI that utilizes Vanilla JavaScript, CodeMirror, and Quill, bundled via Rollup.

### Prerequisites

1.  **Go**: Version **1.26.4** or higher.
2.  **Node.js**: Version **18.x** or higher (needed only if you plan to edit frontend JS bundles).
3.  **Docker**: Needed for container automation testing and isolated sandboxing features.

### Local Installation Steps

1.  **Fork and Clone the Repository**
    ```bash
    git clone https://github.com/your-username/AuraGo.git
    cd AuraGo
    ```

2.  **Install Node Dependencies** (Frontend only)
    ```bash
    npm install
    ```

3.  **Build Frontend Bundles** (Frontend only)
    If you make changes inside the Javascript files or CodeMirror integrations, rebuild the bundled packages:
    ```bash
    npm run build:codemirror
    npm run build:ui
    ```

4.  **Run the Go Backend**
    To run the backend server in development mode:
    ```bash
    go run ./cmd/aurago
    ```

5.  **Build from Source**
    To compile a production binary locally:
    ```bash
    go build -o aurago ./cmd/aurago
    ```

---

## 🧪 Testing Guidelines

Before submitting your code, please run the testing suite to ensure that your modifications do not introduce unexpected behavior or regression issues.

```bash
# Run all Go tests
go test ./...

# Run tests with the race detector
go test -race ./...
```

---

## 💡 How to Contribute

### 1. Reporting Bugs
*   Check if the issue has already been reported in the **[GitHub Issue Tracker](https://github.com/antibyte/AuraGo/issues)**.
*   If not, open a new issue using our **Bug Report Template**.
*   Provide a clear description of the problem, exact reproduction steps, configuration snippets (excluding private API keys!), log errors, and your operating system details.

### 2. Suggesting Features
*   Check existing issues and **[GitHub Discussions](https://github.com/antibyte/AuraGo/discussions)** to ensure your feature hasn't been proposed.
*   Open a new issue using our **Feature Request Template**.
*   Explain the user-facing value: what problem does this feature solve? How would it integrate with existing homelab systems (Docker, Home Assistant, etc.)?

### 3. Submitting Pull Requests
We accept pull requests (PRs) for bug fixes, performance improvements, and new features.

1.  **Create a Branch**
    Create a descriptive branch name starting from the `main` branch:
    ```bash
    git checkout -b feature/your-feature-name
    # OR
    git checkout -b bugfix/issue-number-description
    ```

2.  **Follow Coding Standards**
    *   **Go Guidelines**: Write standard, idiomatic Go code. Run `go fmt ./...`, `go vet ./...`, and `golangci-lint` (if installed) before committing.
    *   **Clean Architecture**: Keep tool interfaces clean. Add new integrations under `internal/tools/` following the existing tool structure.
    *   **Security First**: Never hardcode credentials. Ensure all sensitive configurations are stored in the local secure Vault. Use `LLM Guardian` structures if your tool handles raw external payloads.
    *   **Documentation**: If you change configuration parameters or add a tool, update the corresponding files in `/documentation/configuration.md` or manual guides.

3.  **Write Commit Messages**
    Write clear, descriptive, imperative commit messages. For example:
    *   `feat(tools): add Proxmox VM backup and snapshot support`
    *   `fix(vault): prevent memory leak during decryption key rotation`

4.  **Open the Pull Request**
    *   Push your branch to your forked repository.
    *   Open a Pull Request on the main `AuraGo` repository.
    *   Fill out our **PR Template** in detail.
    *   Ensure all CI checks pass. A maintainer will review your contribution shortly!

---

## 📞 Questions & Support

Have an idea but don't know how to start? Want to discuss design architecture?
*   Head over to **[GitHub Discussions](https://github.com/antibyte/AuraGo/discussions)** and make a post in the Q&A or Ideas category.
*   Join our **[Discord Server](https://discord.gg/aurago)** for real-time collaboration with other homelab maintainers!
