/* api.js - API wrapper, authentication, login modal */

const API = '/api/v1';

function getToken() { return localStorage.getItem('elnssm-token') || ''; }
function setToken(t) { localStorage.setItem('elnssm-token', t); }
function clearToken() { localStorage.removeItem('elnssm-token'); }

function showLoginModal(msg) {
  if (msg) {
    const el = document.getElementById('login-error');
    el.textContent = msg;
    el.style.display = 'block';
  }
  document.getElementById('login-modal').style.display = 'block';
  document.getElementById('login-token').focus();
}

function hideLoginModal() {
  document.getElementById('login-modal').style.display = 'none';
  document.getElementById('login-error').style.display = 'none';
}

async function submitLogin() {
  const token = document.getElementById('login-token').value.trim();
  if (!token) return;
  setToken(token);
  const res = await fetch(API + '/system/version', { headers: { 'Authorization': 'Bearer ' + token } });
  if (res.status === 401) { clearToken(); showLoginModal('Invalid token'); return; }
  hideLoginModal();
  loadServices();
  loadGuardianInfo();
}

function logout() { clearToken(); showLoginModal(); }

async function api(method, path, body) {
  const token = getToken();
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (token) opts.headers['Authorization'] = 'Bearer ' + token;
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(API + path, opts);
  if (res.status === 401) { showLoginModal('Session expired'); return { status: 'error', data: null }; }
  return res.json();
}
