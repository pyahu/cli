---
title: Instalação
description: Instale a Pyahu CLI no macOS, Linux ou Windows e prepare as dependências locais.
---

A Pyahu CLI é um binário único, sem runtime. Toda release é publicada no
[GitHub Releases](https://github.com/pyahu/cli/releases) — macOS, Linux e Windows (amd64 e arm64),
mais o `checksums.txt`. Todos os métodos abaixo baixam de lá.

## Pré-requisitos

A CLI orquestra um cluster k3d local. Você precisa de:

- **Docker** ou **Podman** em execução
- **k3d** `5.x` no `PATH`

A própria CLI não exige `kubectl` nem `helm` no fluxo normal. O comando `pyahu doctor`
verifica essas dependências antes de subir a stack.

## mise (recomendado)

Fixar a CLI junto com o resto do toolchain do projeto é o caminho recomendado: todo mundo do time
fica na mesma versão, e ela fica registrada no repositório.

```bash
# no projeto, escreve em ./mise.toml
mise use "github:pyahu/cli@0.7.0"

# ou para o seu usuário, em qualquer lugar
mise use -g "github:pyahu/cli@0.7.0"

mise install
```

```toml
# mise.toml
[tools]
"github:pyahu/cli" = "0.7.0"
```

O backend `github:` do mise baixa a release do GitHub, confere a atestação do artefato e extrai o
binário — nada de script no meio.

## Pyahu toolchain

A [Pyahu toolchain](https://github.com/pyahu/toolchain) já pina a CLI no profile `cloud`, junto com
k3d, kubectl e o resto do conjunto de Kubernetes. Se você usa a toolchain, já tem a CLI:

```bash
export MISE_ENV=cloud    # coloque no rc do seu shell
mise install
```

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

Auditar antes de executar é simples: `curl -fsSL https://cli.pyahu.io/install.sh` mostra o
conteúdo. Para atualizar, use `pyahu upgrade` (ver abaixo) ou rode o script de novo.

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

## Manter atualizado

A CLI avisa quando está atrás de uma release. Para conferir na hora e atualizar:

```bash
pyahu check-update
pyahu upgrade
```

O `upgrade` confere o SHA-256 da release contra o `checksums.txt` publicado antes de trocar o
binário. Se o `pyahu` foi instalado pelo mise (ou por `go install`), ele não mexe no arquivo —
substituir ali seria desfeito no próximo `mise install` — e mostra o comando certo:

```text
error: this pyahu is managed by another tool; upgrade it with: mise use github:pyahu/cli@0.7.0
```

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
