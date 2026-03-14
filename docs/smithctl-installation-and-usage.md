# smithctl Installation and Usage

`smithctl` is the command-line interface for managing Smith autonomous development loops. It allows you to create, list, inspect, and control loops from your terminal.

## Installation

### From Source

If you have Go installed, you can install `smithctl` directly from the repository:

```bash
go install ./cmd/smithctl
```

Alternatively, you can use the provided `Makefile` to build the binary:

```bash
make build-smithctl
```

This will place the `smithctl` binary in the `bin/` directory.

### Downloading Binaries

In the future, pre-built binaries for various platforms (Linux, macOS, Windows) will be available for download from the [GitHub Releases](https://github.com/smith-org/smith/releases) page.

## Configuration

`smithctl` uses a configuration file located at `~/.smith/config.json` by default. You can also use environment variables or command-line flags to configure it.

### Config File Format

```json
{
  "current_context": "default",
  "contexts": {
    "default": {
      "server": "http://127.0.0.1:8080",
      "token": "your-operator-token"
    }
  }
}
```

### Environment Variables

- `SMITH_API_URL`: The URL of the Smith API server (e.g., `http://localhost:8080`).
- `SMITH_OPERATOR_TOKEN`: Your operator bearer token.
- `SMITH_CONTEXT`: The named configuration context to use.

## Usage

### Global Flags

- `--server URL`: Smith API server URL.
- `--token TOKEN`: Operator bearer token.
- `--config PATH`: Path to smithctl config file (default: `~/.smith/config.json`).
- `--context NAME`: Named config context to use.
- `--output text|json`: Output format (default: `text`).

### Commands

#### Managing Loops

- **List loops:**
  ```bash
  smithctl loop list
  ```

- **Get loop details:**
  ```bash
  smithctl loop get <loop-id>
  ```

- **View loop journal (logs):**
  ```bash
  smithctl loop logs <loop-id>
  ```
  Use `--follow` to stream live journal entries.

- **Create a loop from a GitHub issue:**
  ```bash
  smithctl loop create --title "Fix bug" --source-type github_issue --source-ref "org/repo#123"
  ```

- **Create a loop from a PRD:**
  ```bash
  smithctl loop create --from-prd docs/prd1.md
  ```

- **Cancel a loop:**
  ```bash
  smithctl loop cancel <loop-id> --reason "User requested"
  ```

#### Interactive Control

- **Attach to a running loop:**
  ```bash
  smithctl loop attach <loop-id>
  ```

- **Execute a command in a loop:**
  ```bash
  smithctl loop command <loop-id> --command "ls -la"
  ```

- **Detach from a loop:**
  ```bash
  smithctl loop detach <loop-id>
  ```

#### Managing PRDs

- **Create a PRD template:**
  ```bash
  smithctl prd create "New Feature" --template feature --out docs/feature.md
  ```

- **Submit a PRD to trigger loops:**
  ```bash
  smithctl prd submit --file docs/feature.md
  ```

## Examples

For more examples and detailed API documentation, refer to [docs/loop-ingress-and-cli.md](loop-ingress-and-cli.md).
