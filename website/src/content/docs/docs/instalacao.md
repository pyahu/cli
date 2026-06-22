---
title: Instalação
description: Instale a Pyahu CLI no macOS, Linux ou Windows e prepare as dependências locais.
---

A Pyahu CLI é um binário único, sem runtime. As releases são publicadas no GitHub para
macOS, Linux e Windows (amd64 e arm64).

## Pré-requisitos

A CLI orquestra um cluster k3d local. Você precisa de:

- **Docker** ou **Podman** em execução
- **k3d** `5.x` no `PATH`

A própria CLI não exige `kubectl` nem `helm` no fluxo normal. O comando `pyahu doctor`
verifica essas dependências antes de subir a stack.

## Script de instalação (macOS e Linux)

A forma recomendada. O script detecta o SO e a arquitetura, baixa a release do
GitHub e instala em `/usr/local/bin`:

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh
```

Para instalar em outro diretório (sem `sudo`):

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --bin-dir "$HOME/.local/bin"
```

Para fixar uma versão específica:

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --version v1.2.3
```

Atualizar é rodar o script de novo. Auditar antes de executar também é simples:
`curl -fsSL https://cli.pyahu.io/install.sh` mostra o conteúdo.

## go install

Se você já tem Go `1.26+`:

```bash
go install github.com/pyahu/cli/cmd/pyahu@latest
```

O binário vai para `$(go env GOPATH)/bin`. Garanta que esse diretório está no `PATH`.

## Download manual (GitHub Releases)

Baixe o arquivo da sua plataforma em
[github.com/pyahu/cli/releases](https://github.com/pyahu/cli/releases) e extraia o binário:

```bash
# Linux x86_64
tar -xzf pyahu_Linux_x86_64.tar.gz
sudo mv pyahu /usr/local/bin/

# macOS arm64
tar -xzf pyahu_Darwin_arm64.tar.gz
sudo mv pyahu /usr/local/bin/
```

No Windows, extraia o `.zip` e adicione o `pyahu.exe` ao `PATH`.

## Verificar a instalação

```bash
pyahu --version
pyahu doctor
```

`pyahu doctor` reporta Docker/Podman, k3d, portas locais e a presença de outros clusters
locais. Ele avisa sobre conflitos, mas não falha só porque outro cluster existe.

## Autocomplete do shell

A CLI gera scripts de completion para bash, zsh, fish e powershell:

```bash
# zsh
pyahu completion zsh > "${fpath[1]}/_pyahu"

# bash
pyahu completion bash | sudo tee /etc/bash_completion.d/pyahu > /dev/null

# fish
pyahu completion fish > ~/.config/fish/completions/pyahu.fish
```

Reabra o shell para carregar o completion.

## Próximo passo

```bash
pyahu init --preset platform
pyahu up
```

Veja o passo a passo completo em [Visão geral](/docs/).
