/* service-modal.js - Service add/edit modal with all tabs */

let editingServiceId = null;

const svcTabNames = ['general', 'restart', 'health', 'hooks', 'schedules', 'deps', 'resources'];

function switchTab(name) {
  document.querySelectorAll('#svc-tabs .tab').forEach((t) => {
    t.classList.toggle('active', t.dataset.tab === name);
  });
  svcTabNames.forEach(n => {
    const tc = document.getElementById('tab-' + n);
    if (tc) tc.classList.toggle('active', n === name);
  });
}

function showServiceModal(editId) {
  editingServiceId = editId || null;
  document.getElementById('modal-title').textContent = editId ? 'Edit Service' : 'Add Service';
  document.getElementById('f-id').disabled = !!editId;
  resetServiceForm();
  switchTab('general');
  document.getElementById('service-modal').style.display = 'block';
}

function hideServiceModal() {
  document.getElementById('service-modal').style.display = 'none';
  editingServiceId = null;
}

function resetServiceForm() {
  ['f-id', 'f-description', 'f-exe', 'f-args', 'f-dir', 'f-svc-account', 'f-svc-password',
   'f-rp-retry-window', 'f-rp-cron', 'f-rl-cpu', 'f-rl-cpu-dur', 'f-rl-spike'].forEach(
    id => document.getElementById(id).value = ''
  );
  document.getElementById('f-startup').value = 'manual';
  document.getElementById('f-priority').value = 'normal';
  document.getElementById('f-stop-signal').value = 'ctrl_c';
  document.getElementById('f-stop-timeout').value = '30s';
  document.getElementById('f-start-delay').value = '';
  document.getElementById('f-env').value = '';
  document.getElementById('f-rp-mode').value = 'on_failure';
  document.getElementById('f-rp-delay').value = '5s';
  document.getElementById('f-rp-max-retries').value = '10';
  document.getElementById('f-rp-backoff').value = '2.0';
  document.getElementById('f-rp-max-backoff').value = '5m';
  document.getElementById('f-rp-health-restart').checked = false;
  document.getElementById('f-rl-mem-val').value = '';
  document.getElementById('f-rl-mem-unit').value = 'MB';
  document.getElementById('f-rl-interval').value = '5s';
  resetHealthChecks();
  resetHooks();
  resetSchedules();
  resetDependencies();
  updateStartDelayVisibility();
}

function updateStartDelayVisibility() {
  const type = document.getElementById('f-startup').value;
  document.getElementById('start-delay-group').style.display = type === 'delayed-auto' ? '' : 'none';
}

async function showEditModal(id) {
  showServiceModal(id);
  const res = await api('GET', `/services/${id}`);
  const svc = res.data;
  if (!svc) return;

  document.getElementById('f-id').value = svc.id;
  document.getElementById('f-description').value = svc.description || '';
  document.getElementById('f-exe').value = svc.executable || '';
  document.getElementById('f-args').value = (svc.arguments || []).join(' ');
  document.getElementById('f-dir').value = svc.working_dir || '';
  document.getElementById('f-startup').value = svc.startup_type || 'manual';
  document.getElementById('f-priority').value = svc.priority || 'normal';
  document.getElementById('f-stop-signal').value = svc.stop_signal || 'ctrl_c';
  document.getElementById('f-stop-timeout').value = formatNsDuration(svc.stop_timeout) || '30s';
  document.getElementById('f-start-delay').value = formatNsDuration(svc.start_delay) || '';
  document.getElementById('f-svc-account').value = svc.service_account || '';
  updateStartDelayVisibility();

  const envLines = [];
  if (svc.environment) {
    for (const [k, v] of Object.entries(svc.environment)) envLines.push(k + '=' + v);
  }
  document.getElementById('f-env').value = envLines.join('\n');

  const rp = svc.restart_policy || {};
  document.getElementById('f-rp-mode').value = rp.mode || 'on_failure';
  document.getElementById('f-rp-delay').value = formatNsDuration(rp.delay) || '5s';
  document.getElementById('f-rp-max-retries').value = rp.max_retries || 10;
  document.getElementById('f-rp-backoff').value = rp.backoff_multiplier || 2.0;
  document.getElementById('f-rp-max-backoff').value = formatNsDuration(rp.max_backoff) || '5m';
  document.getElementById('f-rp-retry-window').value = formatNsDuration(rp.retry_window) || '';
  document.getElementById('f-rp-health-restart').checked = !!rp.restart_on_health_fail;
  document.getElementById('f-rp-cron').value = rp.scheduled_restart || '';

  resetHealthChecks();
  if (svc.health_checks) { for (const hc of svc.health_checks) addHealthCheck(hc); }

  const rl = svc.resource_limits || {};
  document.getElementById('f-rl-cpu').value = rl.cpu_threshold || '';
  document.getElementById('f-rl-cpu-dur').value = formatNsDuration(rl.cpu_duration) || '';
  if (rl.memory_max > 0) {
    const gb = rl.memory_max / (1024 * 1024 * 1024);
    if (gb >= 1) {
      document.getElementById('f-rl-mem-val').value = Math.round(gb * 10) / 10;
      document.getElementById('f-rl-mem-unit').value = 'GB';
    } else {
      document.getElementById('f-rl-mem-val').value = Math.round(rl.memory_max / (1024 * 1024));
      document.getElementById('f-rl-mem-unit').value = 'MB';
    }
  } else {
    document.getElementById('f-rl-mem-val').value = '';
    document.getElementById('f-rl-mem-unit').value = 'MB';
  }
  document.getElementById('f-rl-spike').value = rl.memory_spike_ratio || '';
  document.getElementById('f-rl-interval').value = formatNsDuration(rl.check_interval) || '5s';

  loadHooks(svc.hooks);
  loadSchedules(svc.schedules);
  loadDependencies(svc.dependencies);
}

async function submitService() {
  const id = document.getElementById('f-id').value.trim();
  const exe = document.getElementById('f-exe').value.trim();
  if (!id || !exe) { alert('ID and Executable are required'); return; }

  const argsStr = document.getElementById('f-args').value.trim();
  const envStr = document.getElementById('f-env').value.trim();
  const environment = {};
  if (envStr) {
    for (const line of envStr.split('\n')) {
      const eq = line.indexOf('=');
      if (eq > 0) environment[line.substring(0, eq).trim()] = line.substring(eq + 1).trim();
    }
  }

  const body = {
    id,
    name: id,
    description: document.getElementById('f-description').value.trim(),
    executable: exe,
    arguments: argsStr ? argsStr.split(' ') : [],
    working_dir: document.getElementById('f-dir').value.trim(),
    startup_type: document.getElementById('f-startup').value,
    priority: document.getElementById('f-priority').value,
    stop_signal: document.getElementById('f-stop-signal').value,
    stop_timeout: parseDurationToNs(document.getElementById('f-stop-timeout').value.trim()),
    start_delay: parseDurationToNs(document.getElementById('f-start-delay').value.trim()),
    environment: Object.keys(environment).length ? environment : undefined,
    service_account: document.getElementById('f-svc-account').value.trim() || undefined,
    restart_policy: {
      mode: document.getElementById('f-rp-mode').value,
      delay: parseDurationToNs(document.getElementById('f-rp-delay').value.trim()),
      max_retries: parseInt(document.getElementById('f-rp-max-retries').value) || 0,
      backoff_multiplier: parseFloat(document.getElementById('f-rp-backoff').value) || 2.0,
      max_backoff: parseDurationToNs(document.getElementById('f-rp-max-backoff').value.trim()),
      retry_window: parseDurationToNs(document.getElementById('f-rp-retry-window').value.trim()),
      restart_on_health_fail: document.getElementById('f-rp-health-restart').checked,
      scheduled_restart: document.getElementById('f-rp-cron').value.trim() || undefined,
    },
    health_checks: collectHealthChecks(),
    resource_limits: collectResourceLimits(),
    hooks: collectHooks(),
    schedules: collectSchedules(),
    dependencies: collectDependencies(),
  };

  const pw = document.getElementById('f-svc-password').value;
  if (pw) body.service_account_password = pw;

  if (editingServiceId) await api('PUT', `/services/${editingServiceId}`, body);
  else await api('POST', '/services', body);

  hideServiceModal();
  loadServices();
}

function collectResourceLimits() {
  const cpu = parseFloat(document.getElementById('f-rl-cpu').value) || 0;
  const memVal = parseFloat(document.getElementById('f-rl-mem-val').value) || 0;
  const memUnit = document.getElementById('f-rl-mem-unit').value;
  return {
    cpu_threshold: cpu,
    cpu_duration: parseDurationToNs(document.getElementById('f-rl-cpu-dur').value.trim()),
    memory_max: memVal > 0 ? (memUnit === 'GB' ? memVal * 1024 * 1024 * 1024 : memVal * 1024 * 1024) : 0,
    memory_spike_ratio: parseFloat(document.getElementById('f-rl-spike').value) || 0,
    check_interval: parseDurationToNs(document.getElementById('f-rl-interval').value.trim()),
  };
}
