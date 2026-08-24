<template>
  <div class="card stat-card">
    <div class="stat-label">{{ label }}</div>
    <div class="stat-value" :class="toneClass">{{ value }}</div>
    <div v-if="hint" class="stat-hint">{{ hint }}</div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  label: string;
  value: string;
  hint?: string;
  tone?: "ok" | "warn" | "danger" | "neutral";
}>();

const toneClass = computed(() => `tone-${props.tone ?? "neutral"}`);
</script>

<style scoped>
.stat-card {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.stat-label {
  font-size: 0.8rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.stat-value {
  font-size: 2rem;
  font-weight: 700;
}

.tone-ok { color: var(--ok); }
.tone-warn { color: var(--warning); }
.tone-danger { color: var(--danger); }
.tone-neutral { color: var(--text); }

.stat-hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}
</style>
