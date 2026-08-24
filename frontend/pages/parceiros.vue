<template>
  <div class="grid" style="gap: 1.5rem">
    <section class="card">
      <h2 style="margin-top: 0">Token administrativo</h2>
      <p style="color: var(--text-muted); margin-top: 0">
        Os endpoints de cadastro exigem o token administrativo do gateway (variavel ADMIN_TOKEN).
        Ele fica salvo apenas no seu navegador.
      </p>
      <div style="display: flex; gap: 0.75rem">
        <input v-model="token" type="password" placeholder="admin-secret-token" style="flex: 1" />
        <button @click="saveToken">Salvar</button>
      </div>
    </section>

    <section class="card">
      <div style="display: flex; justify-content: space-between; align-items: center">
        <h2 style="margin: 0">Parceiros</h2>
        <span class="tag" :class="partners.length ? 'tag-ok' : 'tag-warn'">{{ partners.length }} carregados</span>
      </div>

      <form class="grid" style="grid-template-columns: 2fr auto; gap: 0.75rem; margin: 1rem 0" @submit.prevent="createPartner">
        <input v-model="newPartnerName" placeholder="Nome do parceiro" required />
        <button type="submit" :disabled="!token">Cadastrar</button>
      </form>

      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Nome</th>
            <th>API key</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in partners" :key="p.id">
            <td>{{ p.id }}</td>
            <td>{{ p.name }}</td>
            <td><code>{{ p.api_key }}</code></td>
            <td>
              <span class="tag" :class="p.active ? 'tag-ok' : 'tag-danger'">
                {{ p.active ? "ativo" : "inativo" }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <section class="card">
      <h2 style="margin-top: 0">Rotas</h2>
      <form class="grid" style="grid-template-columns: 1fr 2fr auto; gap: 0.75rem; margin: 1rem 0" @submit.prevent="createRoute">
        <input v-model="newRoutePrefix" placeholder="/billing" required />
        <input v-model="newRouteUpstream" placeholder="http://mock-billing:9001" required />
        <button type="submit" :disabled="!token">Cadastrar</button>
      </form>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Prefixo</th>
            <th>Upstream</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in routes" :key="r.id">
            <td>{{ r.id }}</td>
            <td>{{ r.path_prefix }}</td>
            <td>{{ r.upstream_url }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <p v-if="errorMessage" style="color: var(--danger)">{{ errorMessage }}</p>
  </div>
</template>

<script setup lang="ts">
interface Partner {
  id: number;
  name: string;
  api_key: string;
  active: boolean;
}

interface Route {
  id: number;
  path_prefix: string;
  upstream_url: string;
}

const config = useRuntimeConfig();
const token = ref("");
const partners = ref<Partner[]>([]);
const routes = ref<Route[]>([]);
const newPartnerName = ref("");
const newRoutePrefix = ref("");
const newRouteUpstream = ref("");
const errorMessage = ref("");

function saveToken() {
  localStorage.setItem("comporta_admin_token", token.value);
  loadAll();
}

function authHeaders() {
  return { Authorization: `Bearer ${token.value}`, "Content-Type": "application/json" };
}

async function loadPartners() {
  const res = await fetch(`${config.public.apiBase}/admin/partners?page=1&page_size=50`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("falha ao carregar parceiros");
  const body = await res.json();
  partners.value = body.data;
}

async function loadRoutes() {
  const res = await fetch(`${config.public.apiBase}/admin/routes?page=1&page_size=50`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("falha ao carregar rotas");
  const body = await res.json();
  routes.value = body.data;
}

async function loadAll() {
  errorMessage.value = "";
  try {
    await Promise.all([loadPartners(), loadRoutes()]);
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : "falha ao carregar dados";
  }
}

async function createPartner() {
  errorMessage.value = "";
  try {
    const res = await fetch(`${config.public.apiBase}/admin/partners`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ name: newPartnerName.value }),
    });
    if (!res.ok) throw new Error((await res.json()).error || "falha ao cadastrar parceiro");
    newPartnerName.value = "";
    await loadPartners();
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : "falha ao cadastrar parceiro";
  }
}

async function createRoute() {
  errorMessage.value = "";
  try {
    const res = await fetch(`${config.public.apiBase}/admin/routes`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({
        path_prefix: newRoutePrefix.value,
        upstream_url: newRouteUpstream.value,
        strip_prefix: true,
      }),
    });
    if (!res.ok) throw new Error((await res.json()).error || "falha ao cadastrar rota");
    newRoutePrefix.value = "";
    newRouteUpstream.value = "";
    await loadRoutes();
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : "falha ao cadastrar rota";
  }
}

onMounted(() => {
  const saved = localStorage.getItem("comporta_admin_token");
  if (saved) {
    token.value = saved;
    loadAll();
  }
});
</script>
