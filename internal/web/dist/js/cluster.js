/* cluster.js - Cluster status and node management */

let clusterRole = 'standalone';

async function loadClusterStatus() {
  try {
    const res = await api('GET', '/cluster/status');
    const d = res.data;
    if (!d) return;
    clusterRole = d.role || 'standalone';
    renderClusterView(d);
  } catch (e) {
    document.getElementById('cluster-content').innerHTML =
      '<div class="empty"><h2>Cluster not configured</h2><p>Set <code>cluster.role</code> in config to "master" or "slave".</p></div>';
  }
}

function renderClusterView(data) {
  const el = document.getElementById('cluster-content');

  if (data.role === 'standalone') {
    el.innerHTML = '<div class="empty"><h2>Standalone Mode</h2><p>This node is not part of a cluster. Configure <code>cluster.role</code> in elnssm.yaml.</p></div>';
    return;
  }

  let html = `
    <div class="cluster-info card">
      <div class="card-header">
        <div>
          <span class="card-title">Node: ${esc(data.node_name || 'unnamed')}</span>
          <span class="status-badge status-running">${esc(data.role)}</span>
        </div>
      </div>
    </div>`;

  if (data.role === 'master' && data.slaves) {
    if (data.slaves.length === 0) {
      html += '<div class="empty" style="padding:30px"><h2>No slave nodes connected</h2></div>';
    } else {
      html += '<h2 style="margin-bottom:12px">Connected Nodes</h2>';
      for (const node of data.slaves) {
        const connected = node.status === 'connected';
        html += `
          <div class="node-card">
            <div style="display:flex;justify-content:space-between;align-items:center">
              <div>
                <span class="node-name">${esc(node.name)}</span>
                <span class="node-status ${connected ? 'connected' : 'disconnected'}">${esc(node.status)}</span>
              </div>
              <div class="card-meta">
                ${esc(node.address)} | v${esc(node.version)} | Last seen: ${formatTimeAgo(node.last_seen)}
              </div>
            </div>
          </div>`;
      }
    }
  }

  if (data.role === 'slave') {
    html += `
      <div class="card">
        <div class="card-meta">
          This node is operating as a <strong>slave</strong> and reports to the master node.
        </div>
      </div>`;
  }

  el.innerHTML = html;
}

function isClusterEnabled() {
  return clusterRole !== 'standalone';
}
