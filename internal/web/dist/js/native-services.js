/* native-services.js - Native Windows Services view */

let nativeServices = [];
let nativeFilter = '';

async function loadNativeServices() {
  try {
    const res = await api('GET', '/native-services');
    if (res.status === 'error') {
      document.getElementById('native-content').innerHTML =
        '<div class="empty"><h2>Windows Services not available</h2><p>Native service management is disabled or not supported.</p></div>';
      return;
    }
    nativeServices = res.data || [];
    renderNativeServices();
  } catch (e) {
    document.getElementById('native-content').innerHTML =
      '<div class="empty"><h2>Windows Services not available</h2><p>Enable <code>enable_native_services: true</code> in config.</p></div>';
  }
}

function renderNativeServices() {
  const el = document.getElementById('native-content');
  const filtered = nativeFilter
    ? nativeServices.filter(s =>
        s.name.toLowerCase().includes(nativeFilter) ||
        s.display_name.toLowerCase().includes(nativeFilter))
    : nativeServices;

  if (filtered.length === 0) {
    el.innerHTML = '<div class="empty"><h2>No services found</h2></div>';
    return;
  }

  el.innerHTML = `
    <table class="native-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Display Name</th>
          <th>Status</th>
          <th>Start Type</th>
          <th>PID</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        ${filtered.map(svc => `
          <tr>
            <td><strong>${esc(svc.name)}</strong></td>
            <td>${esc(svc.display_name)}</td>
            <td><span class="status-badge status-${nativeStatusClass(svc.status)}">${esc(svc.status)}</span></td>
            <td>${esc(svc.start_type)}</td>
            <td>${svc.pid || '-'}</td>
            <td>
              <div class="actions">
                ${svc.status === 'running' ? `
                  <button class="btn btn-danger btn-sm" onclick="nativeSvcAction('${esc(svc.name)}','stop')">Stop</button>
                  <button class="btn btn-secondary btn-sm" onclick="nativeSvcAction('${esc(svc.name)}','restart')">Restart</button>
                ` : `
                  <button class="btn btn-success btn-sm" onclick="nativeSvcAction('${esc(svc.name)}','start')">Start</button>
                `}
              </div>
            </td>
          </tr>
        `).join('')}
      </tbody>
    </table>`;
}

function nativeStatusClass(status) {
  if (status === 'running') return 'running';
  if (status === 'stopped') return 'stopped';
  if (status === 'start_pending' || status === 'continue_pending') return 'starting';
  if (status === 'stop_pending' || status === 'pause_pending') return 'stopping';
  return 'stopped';
}

async function nativeSvcAction(name, action) {
  await api('POST', '/native-services/' + encodeURIComponent(name) + '/' + action);
  setTimeout(loadNativeServices, 1000);
}

function filterNativeServices(value) {
  nativeFilter = value.toLowerCase();
  renderNativeServices();
}
