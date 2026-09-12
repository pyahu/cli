import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://cli.pyahu.io",
  integrations: [
    starlight({
      title: "Pyahu CLI",
      description: "Run local application services on k3d from one project file.",
      logo: {
        src: "./src/assets/pyahu-logo.svg",
        alt: "Pyahu",
        replacesTitle: false
      },
      locales: { root: { label: "English", lang: "en" } },
      customCss: ["./src/styles/starlight.css"],
      social: [{ icon: "github", label: "GitHub", href: "https://github.com/pyahu/cli" }],
      sidebar: [
        {
          label: "Start here",
          items: [
            { label: "Getting started", slug: "docs" },
            { label: "Installation", slug: "docs/installation" },
            { label: "Configuration", slug: "docs/configuration" },
            { label: "Commands", slug: "docs/commands" },
            { label: "Troubleshooting", slug: "docs/troubleshooting" }
          ]
        },
        {
          label: "Guides",
          items: [
            { label: "Kafka Connect & Debezium", slug: "docs/kafka-connect-debezium" },
            { label: "Local certificates", slug: "docs/certificates" },
            { label: "Backup & restore", slug: "docs/backup-restore" }
          ]
        }
      ]
    })
  ]
});
