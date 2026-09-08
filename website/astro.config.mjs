import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://cli.pyahu.io",
  integrations: [
    starlight({
      title: "Pyahu CLI",
      description: "Pyahu CLI documentation for local development infrastructure.",
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
          label: "Introduction",
          items: [
            { label: "Overview", slug: "docs" },
            { label: "Installation", slug: "docs/installation" },
            { label: "Commands", slug: "docs/commands" }
          ]
        },
        {
          label: "Guides",
          items: [
            { label: "Configuration", slug: "docs/configuration" },
            { label: "Kafka Connect & Debezium", slug: "docs/kafka-connect-debezium" },
            { label: "Local certificates", slug: "docs/certificates" },
            { label: "Backup & restore", slug: "docs/backup-restore" }
          ]
        }
      ]
    })
  ]
});
