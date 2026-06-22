---
title: Installation
description: Install the Pyahu CLI on macOS, Linux, or Windows and set up the local dependencies.
---

The Pyahu CLI is a single binary, with no runtime. Releases are published on GitHub for
macOS, Linux, and Windows (amd64 and arm64).

## Prerequisites

The CLI orchestrates a local k3d cluster. You need:

- **Docker** or **Podman** running
- **k3d** `5.x` on your `PATH`

The CLI itself does not require `kubectl` or `helm` in the normal flow. The `pyahu doctor`
command checks these dependencies before bringing up the stack.

## Installation script (macOS and Linux)

The recommended approach. The script detects the OS and architecture, downloads the release from
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

Updating is just running the script again. Auditing it before running is simple too:
`curl -fsSL https://cli.pyahu.io/install.sh` shows the contents.

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

See the full walkthrough in [Overview](/en/docs/).
