<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="brand">Comporta</div>
      <nav class="nav">
        <NuxtLink to="/">Painel</NuxtLink>
        <NuxtLink to="/parceiros">Parceiros e rotas</NuxtLink>
      </nav>
    </header>
    <main class="content">
      <NuxtPage />
    </main>
  </div>
</template>

<style>
.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}

.brand {
  font-weight: 700;
  font-size: 1.25rem;
  letter-spacing: 0.02em;
}

.nav {
  display: flex;
  gap: 1.5rem;
}

.nav a {
  color: var(--text-muted);
  text-decoration: none;
  font-size: 0.95rem;
}

.nav a.router-link-active {
  color: var(--accent);
  font-weight: 600;
}

.content {
  flex: 1;
  padding: 2rem;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
}
</style>
