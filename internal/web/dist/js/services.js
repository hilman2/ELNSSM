/* services.js - Service list rendering and actions */

async function loadServices() {
  const res = await api('GET', '/services');
  const el = document.getElementById('services-list');
  if (!res.data || res.data.length === 0) {
    el.innerHTML = '<div class="empty"><h2>No services registered</h2><p>Click "+ Add Service" to get started.</p></div>';
    return;
  }
  const sorted = res.data.sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id));
  el.innerHTML = sorted.map(svc => `
    <div class="card">
      <div class="card-header">
        <div>
          <span class="card-title">${esc(svc.name || svc.id)}</span>
          <span class="status-badge status-${esc(svc.state)}">${esc(svc.state)}</span>
          ${svc.state === 'running'
            ? `<span class="resource-info" data-svc-res="${esc(svc.id)}">CPU: ${svc.cpu_percent.toFixed(1)}% | RAM: ${formatBytes(svc.memory_bytes)}</span>` : ''}
        </div>
        <div class="actions">
          ${svc.state === 'running' ? `
            <button class="btn btn-danger btn-sm" onclick="svcAction('${esc(svc.id)}','stop')">Stop</button>
            <button class="btn btn-secondary btn-sm" onclick="svcAction('${esc(svc.id)}','restart')">Restart</button>
          ` : `
            <button class="btn btn-success btn-sm" onclick="svcAction('${esc(svc.id)}','start')">Start</button>
          `}
          <button class="btn btn-edit btn-sm" onclick="showEditModal('${esc(svc.id)}')">Edit</button>
          <button class="btn btn-secondary btn-sm" onclick="showPerformanceModal('${esc(svc.id)}')">Perf</button>
          <button class="btn btn-secondary btn-sm" onclick="showLogs('${esc(svc.id)}')">Logs</button>
          <button class="btn btn-danger btn-sm" onclick="removeSvc('${esc(svc.id)}')">Remove</button>
        </div>
      </div>
      <div class="card-meta">
        <strong>Executable:</strong> ${esc(svc.executable)} ${esc((svc.arguments || []).join(' '))}<br>
        ${svc.pid ? `<strong>PID:</strong> ${svc.pid} | ` : ''}
        ${svc.uptime ? `<strong>Uptime:</strong> ${formatDuration(svc.uptime)} | ` : ''}
        <strong>Restarts:</strong> ${svc.restart_count || 0}
        ${svc.dependencies && svc.dependencies.length ? `| <strong>Deps:</strong> ${svc.dependencies.map(d => esc(d.service)).join(', ')}` : ''}
        ${svc.schedules && svc.schedules.length ? `| <strong>Schedules:</strong> ${svc.schedules.length}` : ''}
        ${svc.last_error ? `<br><strong style="color:var(--danger)">Last Error:</strong> ${esc(svc.last_error)}` : ''}
      </div>
    </div>
  `).join('');
}

async function svcAction(id, action) {
  await api('POST', `/services/${id}/${action}`);
  setTimeout(loadServices, 500);
}

async function removeSvc(id) {
  if (!confirm(`Remove service "${id}"?`)) return;
  await api('DELETE', `/services/${id}`);
  loadServices();
}
