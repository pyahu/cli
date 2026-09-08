---
title: Installation
description: Install the Pyahu CLI on macOS, Linux, or Windows and set up the local dependencies.
---

The Pyahu CLI is a single binary, with no runtime. Every release is published to
[GitHub Releases](https://github.com/pyahu/cli/releases) — macOS, Linux, and Windows (amd64 and
arm64), plus `checksums.txt`. Every method below downloads from there.

## Prerequisites

The CLI orchestrates a local k3d cluster. You need:

- **Docker** or **Podman** running
- **k3d** `5.x` on your `PATH`

The CLI itself does not require `kubectl` or `helm` in the normal flow. The `pyahu doctor`
command checks these dependencies before bringing up the stack.

## mise

Pinning the CLI next to the rest of a project's toolchain keeps everyone on the team on the same
version, recorded in the repository:

```bash
# in a project, writes to ./mise.toml
mise use "github:pyahu/cli@0.7.0"

# or for your user, everywhere
mise use -g "github:pyahu/cli@0.7.0"

mise install
```

```toml
# mise.toml
[tools]
"github:pyahu/cli" = "0.7.0"
```

mise's `github:` backend pulls the release from GitHub, verifies the artifact attestation, and
extracts the binary — no script in between.

## Pyahu toolchain

The [Pyahu toolchain](https://github.com/pyahu/toolchain) already pins the CLI in its `cloud`
profile, alongside k3d, kubectl and the rest of the Kubernetes set. If you use the toolchain, you
already have the CLI:

```bash
export MISE_ENV=cloud    # add this to your shell rc
mise install
```

## Installation script (macOS and Linux)

The quickest way onto a single machine. The script detects the OS and architecture, downloads the release from
GitHub, and installs it to `/usr/local/bin`:

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh
```

To install to another directory (without `sudo`):

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --bin-dir "$HOME/.local/bin"
```

To pin a specific version:

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --version v1.2.3
```

Auditing it before running is simple: `curl -fsSL https://cli.pyahu.io/install.sh` shows the
contents. To update, use `pyahu upgrade` (below) or run the script again.

## go install

If you already have Go `1.26+`:

```bash
go install github.com/pyahu/cli/cmd/pyahu@latest
```

The binary goes to `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

## Manual download (GitHub Releases)

Download the archive for your platform from
[github.com/pyahu/cli/releases](https://github.com/pyahu/cli/releases) and extract the binary:

```bash
# Linux x86_64
tar -xzf pyahu_Linux_x86_64.tar.gz
sudo mv pyahu /usr/local/bin/

# macOS arm64
tar -xzf pyahu_Darwin_arm64.tar.gz
sudo mv pyahu /usr/local/bin/
```

On Windows, extract the `.zip` and add `pyahu.exe` to your `PATH`.

## Staying up to date

The CLI warns when it is behind a release. To check on demand and upgrade:

```bash
pyahu check-update
pyahu upgrade
```

`upgrade` verifies the release's SHA-256 against the published `checksums.txt` before swapping the
binary. If `pyahu` was installed by mise (or `go install`), it leaves the file alone — replacing it
would be undone by the next `mise install` — and prints the right command instead:

```text
error: this pyahu is managed by another tool; upgrade it with: mise use github:pyahu/cli@0.7.0
```

## Verify the installation

```bash
pyahu --version
pyahu doctor
```

`pyahu doctor` reports Docker/Podman, k3d, local ports, and the presence of other local
clusters. It warns about conflicts, but does not fail just because another cluster exists.

## Shell autocomplete

The CLI generates completion scripts for bash, zsh, fish, and powershell:

```bash
# zsh
pyahu completion zsh > "${fpath[1]}/_pyahu"

# bash
pyahu completion bash | sudo tee /etc/bash_completion.d/pyahu > /dev/null

# fish
pyahu completion fish > ~/.config/fish/completions/pyahu.fish
```

Reopen your shell to load the completion.

## Next step

```bash
pyahu init --preset platform
pyahu up
```

See the full walkthrough in [Overview](/docs/).
