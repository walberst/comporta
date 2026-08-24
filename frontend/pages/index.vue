<template>
  <div class="grid" style="gap: 1.5rem">
    <section class="grid grid-stats">
      <StatCard
        label="Status do websocket"
        :value="connected ? 'conectado' : 'reconectando'"
        :tone="connected ? 'ok' : 'warn'"
        hint="metricas ao vivo empurradas pelo gateway"
      />
      <StatCard
        label="Requisicoes por segundo"
        :value="rps"
        tone="neutral"
        hint="media da ultima janela recebida"
      />
      <StatCard
        label="Taxa de erro (4xx/5xx)"
        :value="errorRate"
        :tone="errorTone"
        hint="proporcao de respostas com falha"
      />
      <StatCard
        label="Saude do gateway"
        :value="health"
        :tone="healthTone"
        hint="checagem via /healthz"
      />
    </section>

    <section class="card">
      <h2 style="margin-top: 0">Top consumidores por rota</h2>
      <p v-if="!snapshot || snapshot.top_consumers.length === 0" style="color: var(--text-muted)">
        Nenhuma requisicao registrada na janela atual.
      </p>
      <table v-else>
        <thead>
          <tr>
            <th>Parceiro</th>
            <th>Rota</th>
            <th>Requisicoes</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in snapshot.top_consumers" :key="`${c.partner_id}-${c.route_id}`">
            <td>{{ c.partner_name || `parceiro #${c.partner_id}` }}</td>
            <td>{{ c.route_path || `rota #${c.route_id}` }}</td>
            <td>{{ c.requests }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup lang="ts">
const { connected, snapshot } = useMetricsSocket();
const config = useRuntimeConfig();

const rps = computed(() => (snapshot.value ? snapshot.value.requests_per_second.toFixed(1) : "-"));

const errorRate = computed(() =>
  snapshot.value ? `${(snapshot.value.error_rate * 100).toFixed(1)}%` : "-"
);

const errorTone = computed(() => {
  if (!snapshot.value) return "neutral";
  if (snapshot.value.error_rate > 0.1) return "danger";
  if (snapshot.value.error_rate > 0.02) return "warn";
  return "ok";
});

const health = ref("verificando");
const healthTone = ref<"ok" | "warn" | "danger" | "neutral">("neutral");

async function checkHealth() {
  try {
    const res = await fetch(`${config.public.apiBase}/healthz`);
    if (res.ok) {
      health.value = "operacional";
      healthTone.value = "ok";
    } else {
      health.value = "instavel";
      healthTone.value = "warn";
    }
  } catch {
    health.value = "indisponivel";
    healthTone.value = "danger";
  }
}

let healthInterval: ReturnType<typeof setInterval> | null = null;

onMounted(() => {
  checkHealth();
  healthInterval = setInterval(checkHealth, 5000);
});

onBeforeUnmount(() => {
  if (healthInterval) clearInterval(healthInterval);
});
</script>
