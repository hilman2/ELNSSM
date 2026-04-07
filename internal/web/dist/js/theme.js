/* theme.js - Theme switching (light/dark/neo) */

function setTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
  localStorage.setItem('elnssm-theme', t);
  document.querySelectorAll('.theme-btn').forEach(b => b.classList.toggle('active', b.dataset.t === t));
}

(function initTheme() {
  const saved = localStorage.getItem('elnssm-theme');
  if (saved) setTheme(saved);
})();
