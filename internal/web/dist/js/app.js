/* app.js - Main application init, navigation, event bindings */

// --- Navigation ---
let currentView = 'services';

function switchView(view) {
  currentView = view;
  document.querySelectorAll('.nav-btn').forEach(b => b.classList.toggle('active', b.dataset.view === view));
  document.querySelectorAll('.view-panel').forEach(p => p.classList.toggle('active', p.id === 'view-' + view));

  // Load data for the selected view
  if (view === 'services') loadServices();
  else if (view === 'native') loadNativeServices();
  else if (view === 'cluster') loadClusterStatus();
}

// --- Init ---
document.addEventListener('DOMContentLoaded', function() {
  // Login enter key
  document.getElementById('login-token').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') submitLogin();
  });

  // Startup type change -> show/hide start delay
  document.getElementById('f-startup').addEventListener('change', updateStartDelayVisibility);

  // Native services filter
  const nativeFilterEl = document.getElementById('native-filter-input');
  if (nativeFilterEl) {
    nativeFilterEl.addEventListener('input', function() { filterNativeServices(this.value); });
  }

  // Performance range change
  const perfRange = document.getElementById('perf-range');
  if (perfRange) {
    perfRange.addEventListener('change', loadPerformanceData);
  }

  // Check cluster status to show/hide nav button
  checkClusterNav();

  // Initial load
  loadServices();
  loadGuardianInfo();
  connectResourceStream();

  // Refresh intervals
  setInterval(loadServices, 5000);
  setInterval(loadGuardianInfo, 10000);
});

async function checkClusterNav() {
  try {
    const res = await api('GET', '/cluster/status');
    if (res.data && res.data.role !== 'standalone') {
      document.getElementById('nav-cluster').style.display = '';
    }
  } catch (e) { /* cluster nav stays hidden */ }

  // Check if native services are available
  try {
    const res = await api('GET', '/native-services');
    if (res.status === 'ok') {
      document.getElementById('nav-native').style.display = '';
    }
  } catch (e) { /* native nav stays hidden */ }
}
