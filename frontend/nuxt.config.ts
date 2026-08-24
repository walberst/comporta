// Painel roda como SPA pura (sem SSR): ele so consome a API administrativa
// e o websocket de metricas do gateway, entao um servidor Node dedicado so
// para renderizar HTML nao agrega nada e complica o deploy (aqui a imagem
// final e so nginx servindo arquivos estaticos).
export default defineNuxtConfig({
  compatibilityDate: "2025-01-01",
  ssr: false,
  devtools: { enabled: false },

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || "http://localhost:8080",
      wsBase: process.env.NUXT_PUBLIC_WS_BASE || "ws://localhost:8080",
    },
  },

  css: ["~/assets/css/main.css"],

  app: {
    head: {
      title: "Comporta - Painel do Gateway",
      meta: [{ name: "description", content: "Painel de status do gateway Comporta" }],
    },
  },
});
