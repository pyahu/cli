type Service = { name: string; role: string; endpoint: string };
type Step = { number: string; title: string; text: string; command: string };
type Point = { title: string; text: string; icon: "code" | "layers" | "route" | "shield" | "retry" | "review" };

export const content = {
  meta: {
    title: "Pyahu CLI | Local services on k3d",
    description:
      "Run PostgreSQL, Redis, Kafka, RabbitMQ, ZITADEL and Kafka Connect in a local k3d cluster from one project file.",
  },
  nav: {
    why: "Why Pyahu",
    services: "Services",
    docs: "Docs",
    install: "Install",
    menu: "Menu",
  },
  hero: {
    eyebrow: "Local services on k3d",
    title: "Run the services your app needs.",
    accent: "Keep the setup in one file.",
    copy:
      "Choose the services in pyahu.yaml. Pyahu creates the local cluster, waits for it to become ready, and prints the endpoints your app can use.",
    primary: "Start with PostgreSQL",
    secondary: "See configuration",
  },
  quickstart: {
    title: "A working database in three commands",
    items: [
      { number: "01", title: "Create the project file", text: "The default preset contains PostgreSQL only.", command: "pyahu init" },
      { number: "02", title: "Start the stack", text: "Pyahu creates k3d and waits for PostgreSQL.", command: "pyahu up" },
      { number: "03", title: "Load the connection", text: "Use the generated values in your current shell.", command: 'eval "$(pyahu env)"' },
    ] as Step[],
    requirement: "Requires Docker or Podman and k3d 5.x.",
  },
  fit: {
    eyebrow: "When it helps",
    title: "A local stack the whole team can repeat",
    lead:
      "Pyahu is useful when application dependencies have outgrown a few ad-hoc container commands, or when local behavior needs to stay close to Kubernetes.",
    items: [
      { title: "One project file", text: "Services, ports and local defaults live in pyahu.yaml, next to the code that uses them.", icon: "code" },
      { title: "Kubernetes behavior", text: "Ingresses, Secrets, ConfigMaps and persistent volumes are real resources in a local k3d cluster.", icon: "layers" },
      { title: "The full dependency chain", text: "Run database, identity, messaging and CDC together when the application needs all of them.", icon: "route" },
    ] as Point[],
    honest:
      "If you only need a disposable database and do not care about Kubernetes behavior, a single container may be enough. Pyahu is for projects that benefit from a repeatable local cluster.",
  },
  services: {
    eyebrow: "Included services",
    title: "Start small. Add only what the app uses.",
    lead:
      "The minimal preset runs PostgreSQL. The platform preset enables the full stack, and every service can be turned on or off in the same file.",
    items: [
      { name: "PostgreSQL", role: "Databases and optional read replicas", endpoint: "localhost:5432" },
      { name: "ZITADEL", role: "Local identity and OIDC", endpoint: "https://zitadel.localhost" },
      { name: "RabbitMQ", role: "AMQP and management UI", endpoint: "localhost:5672" },
      { name: "Redis", role: "Valkey-compatible data and streams", endpoint: "localhost:6379" },
      { name: "Kafka", role: "Single-broker KRaft for local events", endpoint: "localhost:9092" },
      { name: "Kafka Connect", role: "Connectors, plugins and Debezium CDC", endpoint: "localhost:8083" },
      { name: "Kafka UI", role: "Topics, consumers and connectors", endpoint: "https://kafka-ui.localhost" },
    ] as Service[],
    resourceNote:
      "The full platform preset runs seven real services. Plan for 4 CPU cores, 8 GiB of container memory and 15 GiB of free disk. Use the minimal preset on smaller machines.",
  },
  control: {
    eyebrow: "No hidden cluster",
    title: "Use simple commands. Inspect Kubernetes when you need it.",
    copy:
      "The normal workflow does not require kubectl or Helm. The cluster is still yours: export its kubeconfig and inspect every workload, volume and event when a service needs debugging.",
    link: "Read the command guide",
  },
  safeguards: {
    eyebrow: "Local by default",
    title: "Clear behavior around ports, secrets and data",
    items: [
      { title: "Bound to this machine", text: "Host ports bind to 127.0.0.1 instead of being published to the local network.", icon: "shield" },
      { title: "Secrets stay out of summaries", text: "pyahu up masks passwords and credentials. pyahu env is the explicit command for real values.", icon: "review" },
      { title: "Data is not removed by surprise", text: "pyahu down retains local storage. Permanent removal requires --purge-data and confirmation.", icon: "retry" },
    ] as Point[],
  },
  connect: {
    eyebrow: "Kafka Connect",
    title: "Keep local CDC beside the rest of the stack",
    copy:
      "Declare a Debezium PostgreSQL connector in pyahu.yaml. Pyahu applies it, checks every task, and removes registrations that no longer belong to the file. Custom connector plugins can be downloaded with a required SHA-256 or read from a local jar.",
    link: "Kafka Connect and Debezium guide",
  },
  install: {
    eyebrow: "Install",
    title: "Install the binary, then run doctor",
    lead:
      "Releases are available for macOS, Linux and Windows. The install script and built-in upgrade verify the published checksum.",
    comments: {
      script: "# macOS and Linux",
      mise: "# pin the version in your project",
      go: "# requires Go 1.26+",
    },
    details: "All install methods, prerequisites and shell completion",
    release: "View releases on GitHub",
  },
  final: {
    eyebrow: "Get started",
    title: "Start with PostgreSQL. Add the rest when your app needs it.",
    primary: "Installation guide",
    secondary: "Open the docs",
  },
  footer: {
    tagline: "Local application services on k3d.",
    docs: "Documentation",
    install: "Installation",
    github: "GitHub",
  },
  actions: {
    copy: "Copy command",
    copied: "Copied",
    copyError: "Select and copy the command above.",
  },
};
