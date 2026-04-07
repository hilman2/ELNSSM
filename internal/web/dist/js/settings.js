/* settings.js - System settings modal (Guardian + Cluster) */

function showSettingsModal() {
  document.getElementById('settings-modal').style.display = 'block';
  loadSystemSettings();
}

function hideSettingsModal() {
  document.getElementById('settings-modal').style.display = 'none';
}

async function loadSystemSettings() {
  const res = await api('GET', '/config');
  const d = res.data;
  if (!d) return;

  // Guardian
  const g = d.guardian || {};
  document.getElementById('s-native-services').checked = !!g.enable_native_services;

  // Cluster
  const c = d.cluster || {};
  document.getElementById('s-cluster-role').value = c.role || 'standalone';
  document.getElementById('s-cluster-node-name').value = c.node_name || '';
  document.getElementById('s-cluster-master-addr').value = c.master_addr || '';
  document.getElementById('s-cluster-master-token').value = '';
  document.getElementById('s-cluster-heartbeat').value = c.heartbeat_interval || '30s';
  updateClusterFields();
}

function updateClusterFields() {
  const role = document.getElementById('s-cluster-role').value;
  document.getElementById('cluster-slave-fields').style.display = role === 'slave' ? '' : 'none';
}

async function restartGuardian() {
  if (!confirm('Restart the Guardian service?\n\nRunning child processes will continue and be re-adopted after restart.')) return;
  const btn = document.getElementById('btn-restart-guardian');
  btn.disabled = true;
  btn.textContent = 'Restarting...';
  try {
    await api('POST', '/system/restart');
    alert('Guardian restart initiated. The page will reload shortly.');
    // Poll until the API comes back
    setTimeout(function poll() {
      fetch('/api/v1/system/status').then(r => {
        if (r.ok) location.reload();
        else setTimeout(poll, 2000);
      }).catch(() => setTimeout(poll, 2000));
    }, 3000);
  } catch (e) {
    btn.disabled = false;
    btn.textContent = 'Restart Guardian';
    alert('Failed to restart: ' + (e.message || e));
  }
}

async function saveSystemSettings() {
  const body = {
    guardian: {
      enable_native_services: document.getElementById('s-native-services').checked,
    },
    cluster: {
      role: document.getElementById('s-cluster-role').value,
      node_name: document.getElementById('s-cluster-node-name').value.trim() || undefined,
      master_addr: document.getElementById('s-cluster-master-addr').value.trim() || undefined,
      master_token: document.getElementById('s-cluster-master-token').value.trim() || undefined,
      heartbeat_interval: document.getElementById('s-cluster-heartbeat').value.trim() || '30s',
    },
  };

  const res = await api('PUT', '/config', body);
  hideSettingsModal();
  const msg = res.data && res.data.message ? res.data.message : 'Settings saved.';
  alert(msg);
  // Re-check nav visibility
  checkClusterNav();
}
