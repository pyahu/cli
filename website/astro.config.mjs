import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://cli.pyahu.io",
  integrations: [
    starlight({
      title: "Pyahu CLI",
      description: "Documentação da Pyahu CLI para infraestrutura local de desenvolvimento.",
      logo: {
        src: "./src/assets/pyahu-logo.svg",
        alt: "Pyahu",
        replacesTitle: false
      },
      locales: {
        root: { label: "🇧🇷 Português", lang: "pt-BR" },
        en: { label: "🇺🇸 English", lang: "en" }
      },
      customCss: ["./src/styles/starlight.css"],
      social: [{ icon: "github", label: "GitHub", href: "https://github.com/pyahu/cli" }],
      sidebar: [
        {
          label: "Introdução",
          translations: { en: "Introduction" },
          items: [
            { label: "Visão geral", translations: { en: "Overview" }, slug: "docs" },
            { label: "Instalação", translations: { en: "Installation" }, slug: "docs/instalacao" },
            { label: "Comandos", translations: { en: "Commands" }, slug: "docs/comandos" }
          ]
        },
        {
          label: "Guias",
          translations: { en: "Guides" },
          items: [
            { label: "Configuração", translations: { en: "Configuration" }, slug: "docs/configuracao" },
            { label: "Kafka Connect e Debezium", translations: { en: "Kafka Connect & Debezium" }, slug: "docs/kafka-connect-debezium" },
            { label: "Certificados locais", translations: { en: "Local certificates" }, slug: "docs/certificados" },
            { label: "Backup e restore", translations: { en: "Backup & restore" }, slug: "docs/backup-restore" }
          ]
        }
      ]
    })
  ]
});
