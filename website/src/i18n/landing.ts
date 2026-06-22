export type Lang = "pt-BR" | "en";

type Service = { name: string; role: string; endpoint: string; accent: string };
type Step = { n: string; title: string; text: string; code: string };
type Capability = { label: string; text: string };
type CommandGroup = { title: string; items: string[] };

export const ui = {
  "pt-BR": {
    meta: {
      title: "Pyahu CLI",
      description:
        "Uma CLI para subir Postgres, ZITADEL, RabbitMQ, Kafka, Kafka Connect, Debezium e Kafka UI em k3d com TLS local.",
    },
    nav: { home: "Início", docs: "Documentação" },
    hero: {
      eyebrow: "infra local para desenvolvimento",
      titleLead: "Sua stack local em",
      titleAccent: "um só comando.",
      copy: "Pyahu CLI provisiona PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect, Debezium e Kafka UI em um cluster k3d, com TLS local e endpoints previsíveis. Sem transformar o setup em um projeto paralelo.",
      ctaStart: "Começar agora",
      ctaCommands: "Ver comandos",
      meta: "macOS e Linux · requer Docker ou Podman + k3d",
      terminalAlt: "Terminal mostrando pyahu up e pyahu services",
      term: {
        preflight: "Docker, k3d e portas locais",
        cluster: "k3d pyahu-local pronto",
        certsApplied: "aplicado",
        services: "postgres · zitadel · rabbitmq · kafka",
      },
    },
    services: {
      eyebrow: "o que sobe",
      title: "Seis serviços, prontos para desenvolver e testar.",
      lead: "Tudo roda local em k3d. Os endpoints batem com o que os apps já esperam: convenção sobre configuração, sem reconfigurar nada.",
      items: [
        { name: "PostgreSQL", role: "Banco relacional primário, com réplicas de leitura opcionais.", endpoint: "localhost:5432", accent: "brand" },
        { name: "ZITADEL", role: "Identidade e OIDC em HTTPS local, sem CA pública.", endpoint: "zitadel.localhost", accent: "indigo" },
        { name: "RabbitMQ", role: "Mensageria AMQP com console de management embutido.", endpoint: "localhost:5672", accent: "amber" },
        { name: "Kafka", role: "Broker em modo KRaft para streaming de eventos.", endpoint: "localhost:9092", accent: "brand" },
        { name: "Kafka Connect + Debezium", role: "CDC declarativo do Postgres direto no pyahu.yaml.", endpoint: "localhost:8083", accent: "cyan" },
        { name: "Kafka UI", role: "Inspeção visual de tópicos, conectores e consumers.", endpoint: "kafka-ui.localhost", accent: "indigo" },
      ] as Service[],
    },
    platform: {
      eyebrow: "plataforma pyahu",
      title: "Kubernetes de verdade, não uma abstração.",
      copy: "A Pyahu CLI é a porta de entrada local para a <strong>Plataforma Pyahu</strong>: um subconjunto fiel da experiência real, rodando na sua máquina. É k3d/k3s de verdade, com Traefik, PersistentVolumes, ConfigMaps e Secrets. A CLI não esconde o Kubernetes do dev. Ela só facilita o provisionamento e a operação, e o cluster continua seu para inspecionar quando quiser.",
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
      lead: "Binário único, sem runtime. Releases assinadas no GitHub para macOS, Linux e Windows.",
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
      copy: "Declare o conector no <code>pyahu.yaml</code>. A CLI renderiza o JSON do Connect, grava em um Secret e aplica via REST API, idempotente a cada <code>pyahu up</code>.",
      link: "Guia de Kafka Connect →",
    },
    capabilities: {
      eyebrow: "por que pyahu",
      title: "Simples por design, do boot ao teardown.",
      items: [
        { label: "Stack local completa", text: "k3d + manifests gerados pela CLI. Sem kubectl ou helm no fluxo normal." },
        { label: "Auth em HTTPS local", text: "ZITADEL em zitadel.localhost com CA própria e trust explícito no host." },
        { label: "CDC sem cerimônia", text: "Debezium para Postgres declarado no YAML. A CLI renderiza e aplica o conector." },
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
      title: "Pyahu CLI",
      description:
        "A CLI to spin up Postgres, ZITADEL, RabbitMQ, Kafka, Kafka Connect, Debezium and Kafka UI on k3d with local TLS.",
    },
    nav: { home: "Home", docs: "Documentation" },
    hero: {
      eyebrow: "local infrastructure for development",
      titleLead: "Your local stack in",
      titleAccent: "a single command.",
      copy: "Pyahu CLI provisions PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect, Debezium and Kafka UI on a k3d cluster, with local TLS and predictable endpoints. Without turning your setup into a side project.",
      ctaStart: "Get started",
      ctaCommands: "See commands",
      meta: "macOS and Linux · requires Docker or Podman + k3d",
      terminalAlt: "Terminal showing pyahu up and pyahu services",
      term: {
        preflight: "Docker, k3d and local ports",
        cluster: "k3d pyahu-local ready",
        certsApplied: "applied",
        services: "postgres · zitadel · rabbitmq · kafka",
      },
    },
    services: {
      eyebrow: "what runs",
      title: "Six services, ready to build and test.",
      lead: "Everything runs locally on k3d. The endpoints match what apps already expect: convention over configuration, with nothing to reconfigure.",
      items: [
        { name: "PostgreSQL", role: "Primary relational database, with optional read replicas.", endpoint: "localhost:5432", accent: "brand" },
        { name: "ZITADEL", role: "Identity and OIDC over local HTTPS, no public CA.", endpoint: "zitadel.localhost", accent: "indigo" },
        { name: "RabbitMQ", role: "AMQP messaging with a built-in management console.", endpoint: "localhost:5672", accent: "amber" },
        { name: "Kafka", role: "Event streaming broker in KRaft mode.", endpoint: "localhost:9092", accent: "brand" },
        { name: "Kafka Connect + Debezium", role: "Declarative Postgres CDC straight from pyahu.yaml.", endpoint: "localhost:8083", accent: "cyan" },
        { name: "Kafka UI", role: "Visual inspection of topics, connectors and consumers.", endpoint: "kafka-ui.localhost", accent: "indigo" },
      ] as Service[],
    },
    platform: {
      eyebrow: "pyahu platform",
      title: "Real Kubernetes, not an abstraction.",
      copy: "Pyahu CLI is the local on-ramp to the <strong>Pyahu Platform</strong>: a faithful subset of the real experience, running on your machine. It is real k3d/k3s, with Traefik, PersistentVolumes, ConfigMaps and Secrets. The CLI does not hide Kubernetes from the developer. It just makes provisioning and operation easier, and the cluster stays yours to inspect whenever you want.",
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
      lead: "A single binary, no runtime. Signed releases on GitHub for macOS, Linux and Windows.",
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
      copy: "Declare the connector in <code>pyahu.yaml</code>. The CLI renders the Connect JSON, stores it in a Secret and applies it via the REST API, idempotent on every <code>pyahu up</code>.",
      link: "Kafka Connect guide →",
    },
    capabilities: {
      eyebrow: "why pyahu",
      title: "Simple by design, from boot to teardown.",
      items: [
        { label: "Complete local stack", text: "k3d + manifests generated by the CLI. No kubectl or helm in the normal flow." },
        { label: "Auth over local HTTPS", text: "ZITADEL on zitadel.localhost with its own CA and explicit host trust." },
        { label: "CDC without ceremony", text: "Debezium for Postgres declared in YAML. The CLI renders and applies the connector." },
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
