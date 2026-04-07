/* resources.js - Host resource WebSocket stream + Guardian info */

let resWs = null;
let resWsBackoff = 5000;

function updateHostDisplay(cpu, memTotal, memUsed, memPct) {
  const el = document.getElementById('host-resources');
  if (!el) return;
  el.textContent = 'CPU: ' + cpu.toFixed(1) + '% | RAM: ' + formatBytes(memUsed) + ' / ' + formatBytes(memTotal) + ' (' + memPct.toFixed(0) + '%)';
}

function connectResourceStream() {
  const token = getToken();
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsToken = token ? '?token=' + encodeURIComponent(token) : '';
  resWs = new WebSocket(proto + '//' + location.host + API + '/system/resources/stream' + wsToken);

  resWs.onopen = function() { resWsBackoff = 5000; };

  resWs.onmessage = function(evt) {
    try {
      const snap = JSON.parse(evt.data);
      if (snap.host) {
        updateHostDisplay(snap.host.cpu_percent, snap.host.memory_total, snap.host.memory_used, snap.host.memory_percent);
      }
      if (snap.services) {
        for (const svc of snap.services) {
          const badge = document.querySelector('[data-svc-res="' + CSS.escape(svc.id) + '"]');
          if (badge) {
            badge.textContent = 'CPU: ' + svc.cpu_percent.toFixed(1) + '% | RAM: ' + formatBytes(svc.memory_bytes);
          }
        }
      }
    } catch (e) { /* ignore */ }
  };

  resWs.onclose = function() {
    resWs = null;
    setTimeout(connectResourceStream, resWsBackoff);
    resWsBackoff = Math.min(resWsBackoff * 2, 60000);
  };

  resWs.onerror = function() { if (resWs) resWs.close(); };
}

async function loadGuardianInfo() {
  try {
    const res = await api('GET', '/system/status');
    const d = res.data;
    document.getElementById('guardian-info').textContent =
      'Guardian: ' + d.version + ' | Uptime: ' + d.uptime + ' | Services: ' + d.services_total;
    if (d.host_cpu_percent !== undefined) {
      updateHostDisplay(d.host_cpu_percent, d.host_memory_total, d.host_memory_used, d.host_memory_percent);
    }
  } catch (e) {
    document.getElementById('guardian-info').textContent = 'Guardian: disconnected';
  }
}
