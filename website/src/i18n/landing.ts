export type Lang = "pt-BR" | "en";

type Service = { name: string; role: string; endpoint: string; accent: string };
type Step = { n: string; title: string; text: string; code: string };
type Capability = { label: string; text: string };
type CommandGroup = { title: string; items: string[] };

export const ui = {
  "pt-BR": {
    meta: {
      title: "Pyahu CLI | Sua stack local. Pronta para desenvolver.",
      description:
        "Uma CLI para subir Postgres, ZITADEL, RabbitMQ, Redis, Kafka, Kafka Connect, Debezium e Kafka UI em k3d com TLS local.",
    },
    nav: { home: "Início", docs: "Documentação" },
    hero: {
      eyebrow: "infra local para desenvolvimento",
      titleLead: "Sua stack local.",
      titleAccent: "Pronta para desenvolver.",
      copy: "Banco de dados, identidade e mensageria em um cluster Kubernetes na sua máquina. Uma stack reproduzível para desenvolver, integrar e validar — sem transformar o setup em um projeto paralelo.",
      ctaStart: "Começar agora",
      ctaCommands: "Ver comandos",
      meta: "macOS e Linux · requer Docker ou Podman + k3d",
      terminalAlt: "Terminal mostrando pyahu up e pyahu services",
      term: {
        preflight: "Docker, k3d e portas locais",
        cluster: "k3d pyahu-local pronto",
        certsApplied: "aplicado",
        services: "postgres · zitadel · rabbitmq · redis · kafka",
      },
    },
    services: {
      eyebrow: "o que sobe",
      title: "Sete serviços, prontos para desenvolver e testar.",
      lead: "Escolha os serviços no pyahu.yaml e conecte sua aplicação aos endpoints locais. Tudo roda em k3d, com configuração versionável e recursos que você pode inspecionar.",
      items: [
        { name: "PostgreSQL", role: "Banco relacional primário, com réplicas de leitura opcionais.", endpoint: "localhost:5432", accent: "brand" },
        { name: "ZITADEL", role: "Identidade e OIDC em HTTPS local, sem CA pública.", endpoint: "zitadel.localhost", accent: "indigo" },
        { name: "RabbitMQ", role: "Mensageria AMQP com console de management embutido.", endpoint: "localhost:5672", accent: "amber" },
        { name: "Redis", role: "Valkey com AOF ligado por padrão, para estado quente e streams.", endpoint: "localhost:6379", accent: "cyan" },
        { name: "Kafka", role: "Broker em modo KRaft para streaming de eventos.", endpoint: "localhost:9092", accent: "brand" },
        { name: "Kafka Connect + Debezium", role: "CDC declarativo do Postgres direto no pyahu.yaml.", endpoint: "localhost:8083", accent: "cyan" },
        { name: "Kafka UI", role: "Inspeção visual de tópicos, conectores e consumers.", endpoint: "kafka-ui.localhost", accent: "indigo" },
      ] as Service[],
    },
    platform: {
      eyebrow: "plataforma pyahu",
      title: "A simplicidade de uma CLI.\nO controle do Kubernetes.",
      copy: "A CLI cuida do provisionamento e da operação da stack local. Por baixo, você encontra <strong>k3d/k3s, Traefik e recursos Kubernetes</strong>: volumes, ConfigMaps e Secrets. Use os comandos da Pyahu no dia a dia e inspecione o cluster quando precisar.",
      link: "Conheça a stack local →",
    },
    steps: {
      eyebrow: "começo rápido",
      title: "Do zero ao cluster rodando em três passos.",
      items: [
        { n: "01", title: "Gere a stack", text: "Um preset escreve um pyahu.yaml legível com os serviços, portas e credenciais locais.", code: "pyahu init --preset platform" },
        { n: "02", title: "Suba o cluster", text: "A CLI valida dependências, cria o k3d e reconcilia os recursos Kubernetes de forma idempotente.", code: "pyahu up" },
        { n: "03", title: "Conecte os apps", text: "Endpoints previsíveis em localhost e variáveis de ambiente prontas para colar.", code: 'eval "$(pyahu env)"' },
      ] as Step[],
    },
    install: {
      eyebrow: "instalação",
      title: "Escolha como instalar a CLI.",
      lead: "Binário único, sem runtime. Releases no GitHub para macOS, Linux e Windows.",
      comments: {
        script: "# macOS e Linux · instala em /usr/local/bin",
        go: "# requer Go 1.26+",
        release: "# baixe o tar da release e extraia",
      },
      foot: 'Detalhes, verificação e autocomplete em <a href="$DOCS/instalacao">Instalação</a>.',
    },
    tls: {
      eyebrow: "localhost com TLS",
      title: "HTTPS local de verdade, sem CA pública.",
      copy: "A CLI gera uma CA local, emite o certificado para <code>localhost</code> e <code>*.localhost</code> (que cobre <code>zitadel.localhost</code>, <code>kafka-ui.localhost</code>…), grava o Secret TLS no Kubernetes e deixa o trust do host explícito, em um comando.",
      link: "Como funcionam os certificados →",
    },
    cdc: {
      eyebrow: "change data capture",
      title: "Debezium declarado no YAML, aplicado no up.",
      copy: "Declare conector <em>e plugin</em> no <code>pyahu.yaml</code>. A CLI baixa o artefato conferindo o sha256, instala no worker e aplica o conector via REST API, idempotente a cada <code>pyahu up</code>. Conector cuja fonte só existe depois da app subir vai como <code>optional</code> e entra depois com <code>pyahu connectors apply</code>.",
      link: "Guia de Kafka Connect →",
    },
    capabilities: {
      eyebrow: "por que pyahu",
      title: "Simples por design, do boot ao teardown.",
      items: [
        { label: "Stack local completa", text: "k3d + manifests gerados pela CLI. Sem kubectl ou helm no fluxo normal." },
        { label: "Auth em HTTPS local", text: "ZITADEL em zitadel.localhost com CA própria e trust explícito no host." },
        { label: "CDC sem cerimônia", text: "Debezium para Postgres declarado no YAML. A CLI renderiza e aplica o conector." },
        { label: "Plugins do Connect por declaração", text: "URL com sha256 ou jar local instalados no worker. Sem construir imagem para cada connector." },
        { label: "Alarme por task", text: "pyahu connectors status olha cada task: connector fica RUNNING com a task FAILED, e é assim que outbox para em silêncio." },
        { label: "Backup direto", text: "Dumps reais do Postgres para o disco do host; restore de arquivo local ou S3." },
        { label: "Configuração mínima", text: "Um pyahu.yaml define serviços, portas e credenciais. O resto fica nos comandos." },
      ] as Capability[],
    },
    commands: {
      eyebrow: "referência",
      title: "Um verbo para cada etapa do ciclo local.",
      groups: [
        { title: "Ciclo de vida", items: ["pyahu init", "pyahu up", "pyahu down", "pyahu doctor"] },
        { title: "Inspeção", items: ["pyahu status", "pyahu services", "pyahu describe", "pyahu logs"] },
        { title: "Conexão e dados", items: ["pyahu env", "pyahu kubeconfig", "pyahu backup", "pyahu restore"] },
        { title: "TLS local", items: ["pyahu certs status", "pyahu certs trust", "pyahu certs rotate"] },
      ] as CommandGroup[],
      cta: "Referência completa de comandos",
    },
    finalCta: {
      eyebrow: "pronto para começar",
      title: "A infraestrutura local fica pronta. O time foca no produto.",
      ctaInstall: "Instalar a Pyahu CLI",
      ctaDocs: "Abrir documentação",
    },
    footer: {
      tagline: "Infraestrutura local para desenvolvimento.",
      links: { docs: "Documentação", install: "Instalação", commands: "Comandos" },
      legal: "Infraestrutura local para desenvolvimento",
    },
  },

  en: {
    meta: {
      title: "Pyahu CLI | Your local stack. Ready to build.",
      description:
        "A CLI to spin up Postgres, ZITADEL, RabbitMQ, Redis, Kafka, Kafka Connect, Debezium and Kafka UI on k3d with local TLS.",
    },
    nav: { home: "Home", docs: "Documentation" },
    hero: {
      eyebrow: "local infrastructure for development",
      titleLead: "Your local stack.",
      titleAccent: "Ready to build.",
      copy: "Databases, identity and messaging in a Kubernetes cluster on your machine. A reproducible stack to develop, integrate and validate — without turning setup into a side project.",
      ctaStart: "Get started",
      ctaCommands: "See commands",
      meta: "macOS and Linux · requires Docker or Podman + k3d",
      terminalAlt: "Terminal showing pyahu up and pyahu services",
      term: {
        preflight: "Docker, k3d and local ports",
        cluster: "k3d pyahu-local ready",
        certsApplied: "applied",
        services: "postgres · zitadel · rabbitmq · redis · kafka",
      },
    },
    services: {
      eyebrow: "what runs",
      title: "Seven services, ready to build and test.",
      lead: "Choose services in pyahu.yaml and connect your application to their local endpoints. Everything runs on k3d, with versionable configuration and resources you can inspect.",
      items: [
        { name: "PostgreSQL", role: "Primary relational database, with optional read replicas.", endpoint: "localhost:5432", accent: "brand" },
        { name: "ZITADEL", role: "Identity and OIDC over local HTTPS, no public CA.", endpoint: "zitadel.localhost", accent: "indigo" },
        { name: "RabbitMQ", role: "AMQP messaging with a built-in management console.", endpoint: "localhost:5672", accent: "amber" },
        { name: "Redis", role: "Valkey with AOF on by default, for hot state and streams.", endpoint: "localhost:6379", accent: "cyan" },
        { name: "Kafka", role: "Event streaming broker in KRaft mode.", endpoint: "localhost:9092", accent: "brand" },
        { name: "Kafka Connect + Debezium", role: "Declarative Postgres CDC straight from pyahu.yaml.", endpoint: "localhost:8083", accent: "cyan" },
        { name: "Kafka UI", role: "Visual inspection of topics, connectors and consumers.", endpoint: "kafka-ui.localhost", accent: "indigo" },
      ] as Service[],
    },
    platform: {
      eyebrow: "pyahu platform",
      title: "The simplicity of a CLI.\nThe control of Kubernetes.",
      copy: "The CLI handles local stack provisioning and operation. Underneath, you have <strong>k3d/k3s, Traefik and Kubernetes resources</strong>: volumes, ConfigMaps and Secrets. Use Pyahu commands every day and inspect the cluster whenever you need to.",
      link: "Explore the local stack →",
    },
    steps: {
      eyebrow: "quick start",
      title: "From zero to a running cluster in three steps.",
      items: [
        { n: "01", title: "Generate the stack", text: "A preset writes a readable pyahu.yaml with the local services, ports and credentials.", code: "pyahu init --preset platform" },
        { n: "02", title: "Bring up the cluster", text: "The CLI validates dependencies, creates k3d and reconciles the Kubernetes resources idempotently.", code: "pyahu up" },
        { n: "03", title: "Connect your apps", text: "Predictable endpoints on localhost and connection env vars ready to paste.", code: 'eval "$(pyahu env)"' },
      ] as Step[],
    },
    install: {
      eyebrow: "installation",
      title: "Choose how to install the CLI.",
      lead: "A single binary, no runtime. Releases on GitHub for macOS, Linux and Windows.",
      comments: {
        script: "# macOS and Linux · installs to /usr/local/bin",
        go: "# requires Go 1.26+",
        release: "# download the release tarball and extract it",
      },
      foot: 'Details, verification and shell completion in <a href="$DOCS/instalacao">Installation</a>.',
    },
    tls: {
      eyebrow: "localhost with TLS",
      title: "Real local HTTPS, no public CA.",
      copy: "The CLI generates a local CA, issues the certificate for <code>localhost</code> and <code>*.localhost</code> (which covers <code>zitadel.localhost</code>, <code>kafka-ui.localhost</code>…), stores the TLS Secret in Kubernetes and makes host trust explicit, in one command.",
      link: "How certificates work →",
    },
    cdc: {
      eyebrow: "change data capture",
      title: "Debezium declared in YAML, applied on up.",
      copy: "Declare the connector <em>and its plugins</em> in <code>pyahu.yaml</code>. The CLI downloads the artifact against its sha256, installs it into the worker and applies the connector via the REST API, idempotent on every <code>pyahu up</code>. A connector whose source only exists after the app boots goes in as <code>optional</code> and lands later with <code>pyahu connectors apply</code>.",
      link: "Kafka Connect guide →",
    },
    capabilities: {
      eyebrow: "why pyahu",
      title: "Simple by design, from boot to teardown.",
      items: [
        { label: "Complete local stack", text: "k3d + manifests generated by the CLI. No kubectl or helm in the normal flow." },
        { label: "Auth over local HTTPS", text: "ZITADEL on zitadel.localhost with its own CA and explicit host trust." },
        { label: "CDC without ceremony", text: "Debezium for Postgres declared in YAML. The CLI renders and applies the connector." },
        { label: "Connect plugins by declaration", text: "A URL with its sha256, or a local jar, installed into the worker. No image to build per connector." },
        { label: "Alerting per task", text: "pyahu connectors status reads every task: a connector stays RUNNING with its task FAILED, and that is how an outbox stops in silence." },
        { label: "Backups, direct", text: "Real Postgres dumps to the host disk; restore from a local file or S3." },
        { label: "Minimal configuration", text: "One pyahu.yaml defines services, ports and credentials. The rest lives in the commands." },
      ] as Capability[],
    },
    commands: {
      eyebrow: "reference",
      title: "One verb for each step of the local cycle.",
      groups: [
        { title: "Lifecycle", items: ["pyahu init", "pyahu up", "pyahu down", "pyahu doctor"] },
        { title: "Inspection", items: ["pyahu status", "pyahu services", "pyahu describe", "pyahu logs"] },
        { title: "Connection & data", items: ["pyahu env", "pyahu kubeconfig", "pyahu backup", "pyahu restore"] },
        { title: "Local TLS", items: ["pyahu certs status", "pyahu certs trust", "pyahu certs rotate"] },
      ] as CommandGroup[],
      cta: "Full command reference",
    },
    finalCta: {
      eyebrow: "ready to start",
      title: "The local infrastructure is ready. The team focuses on the product.",
      ctaInstall: "Install the Pyahu CLI",
      ctaDocs: "Open the documentation",
    },
    footer: {
      tagline: "Local infrastructure for development.",
      links: { docs: "Documentation", install: "Installation", commands: "Commands" },
      legal: "Local infrastructure for development",
    },
  },
} satisfies Record<Lang, unknown>;

export const experience = {
  "pt-BR": {
    menu: "Menu", services: "A stack", workflow: "Como funciona", install: "Instalar", platform: "Pyahu Platform",
    sample: "EXEMPLO DE SESSÃO LOCAL", open: "COMMUNITY · OPEN SOURCE", ready: "Seu ambiente de desenvolvimento, sob seu controle.",
    principles: ["Configuração versionável", "Serviços reais, localmente", "Kubernetes inspecionável"],
    copy: "Copiar comando", copied: "Copiado!", copyError: "Selecione o comando para copiar.",
    ecosystem: { eyebrow: "PARTE DA PYAHU PLATFORM", title: "O caminho para produção\ncomeça na sua máquina.", body: "A CLI é a base local da suíte Pyahu. Prepare as ferramentas com a Toolchain, desenvolva e valide com a CLI e evolua para a infraestrutura que seu time precisa.", action: "Conheça a plataforma", local: "Desenvolva e valide", prepare: "Prepare o ambiente", operate: "Evolua a operação", soon: "AI Factory e Initializer · Em breve" },
    release: "Baixar no GitHub", requirement: "Antes de começar: Docker ou Podman em execução e k3d no PATH.",
  },
  en: {
    menu: "Menu", services: "The stack", workflow: "How it works", install: "Install", platform: "Pyahu Platform",
    sample: "EXAMPLE LOCAL SESSION", open: "COMMUNITY · OPEN SOURCE", ready: "Your development environment, under your control.",
    principles: ["Versionable configuration", "Real services, locally", "Inspectable Kubernetes"],
    copy: "Copy command", copied: "Copied!", copyError: "Select the command to copy it.",
    ecosystem: { eyebrow: "PART OF PYAHU PLATFORM", title: "The path to production\nstarts on your machine.", body: "The CLI is the local foundation of the Pyahu suite. Prepare your tools with Toolchain, develop and validate with the CLI, and grow into the infrastructure your team needs.", action: "Explore the platform", local: "Develop and validate", prepare: "Prepare your tools", operate: "Grow your operations", soon: "AI Factory and Initializer · Coming soon" },
    release: "Download on GitHub", requirement: "Before you start: Docker or Podman running and k3d on your PATH.",
  },
};
