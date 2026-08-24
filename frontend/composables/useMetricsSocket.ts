// Composable que mantem a conexao com o websocket de metricas ao vivo do
// gateway. A reconexao automatica existe porque o painel fica aberto por
// longos periodos numa tela de operacao e nao pode exigir refresh manual
// sempre que o gateway reinicia (deploy, restart do container etc).

export interface ConsumerCount {
  partner_id: number;
  partner_name: string;
  route_id: number;
  route_path: string;
  requests: number;
}

export interface MetricsSnapshot {
  timestamp: string;
  requests_per_second: number;
  error_rate: number;
  top_consumers: ConsumerCount[];
}

export function useMetricsSocket() {
  const config = useRuntimeConfig();
  const connected = ref(false);
  const snapshot = ref<MetricsSnapshot | null>(null);

  let socket: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  const reconnectDelayMs = 2000;

  function connect() {
    const url = `${config.public.wsBase}/ws`;
    socket = new WebSocket(url);

    socket.onopen = () => {
      connected.value = true;
    };

    socket.onmessage = (event) => {
      try {
        snapshot.value = JSON.parse(event.data) as MetricsSnapshot;
      } catch {
        // Mensagem fora do formato esperado e ignorada silenciosamente: o
        // proximo snapshot valido corrige o painel sozinho.
      }
    };

    socket.onclose = () => {
      connected.value = false;
      scheduleReconnect();
    };

    socket.onerror = () => {
      socket?.close();
    };
  }

  function scheduleReconnect() {
    if (reconnectTimer) return;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, reconnectDelayMs);
  }

  onMounted(connect);

  onBeforeUnmount(() => {
    if (reconnectTimer) clearTimeout(reconnectTimer);
    socket?.close();
  });

  return { connected, snapshot };
}
