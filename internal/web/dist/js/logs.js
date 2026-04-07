/* logs.js - Log modal with WebSocket streaming */

let logWs = null;

function showLogs(id) {
  document.getElementById('log-modal').style.display = 'block';
  document.getElementById('log-title').textContent = 'Logs: ' + id;
  const el = document.getElementById('log-content');
  el.textContent = 'Connecting...\n';

  const token = getToken();
  const authHeaders = token ? { 'Authorization': 'Bearer ' + token } : {};
  fetch(API + '/services/' + id + '/logs?lines=200', { headers: authHeaders }).then(r => {
    if (r.status === 401) { showLoginModal('Session expired'); return ''; }
    return r.text();
  }).then(text => {
    el.innerHTML = linkifyUrls(text || '(no logs yet)\n');
    el.scrollTop = el.scrollHeight;
  });

  if (logWs) logWs.close();
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsToken = token ? '&token=' + encodeURIComponent(token) : '';
  logWs = new WebSocket(proto + '//' + location.host + API + '/services/' + id + '/logs/stream?stream=combined' + wsToken);
  logWs.onmessage = (e) => {
    const msg = JSON.parse(e.data);
    el.innerHTML += linkifyUrls(msg.line + '\n');
    el.scrollTop = el.scrollHeight;
  };
}

function hideLogModal() {
  document.getElementById('log-modal').style.display = 'none';
  if (logWs) { logWs.close(); logWs = null; }
}
