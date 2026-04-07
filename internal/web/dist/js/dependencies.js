/* dependencies.js - Dependency management tab in service modal */

let depCount = 0;

function resetDependencies() {
  depCount = 0;
  document.getElementById('deps-list').innerHTML = '';
}

function addDependency(dep) {
  const idx = depCount++;
  const list = document.getElementById('deps-list');
  const div = document.createElement('div');
  div.className = 'hc-item';
  div.id = 'dep-' + idx;
  div.innerHTML = `
    <button class="hc-remove" onclick="removeDependency(${idx})">Remove</button>
    <div class="form-row">
      <div class="form-group"><label>Service ID</label>
        <input type="text" id="dep-svc-${idx}" placeholder="postgres">
      </div>
      <div class="form-group"><label>Condition</label>
        <select id="dep-type-${idx}">
          <option value="running">Running</option>
          <option value="healthy">Healthy</option>
        </select>
      </div>
    </div>
    <div class="form-group"><label>Timeout</label>
      <input type="text" id="dep-timeout-${idx}" value="60s" placeholder="60s">
    </div>`;
  list.appendChild(div);
  if (dep) {
    document.getElementById('dep-svc-' + idx).value = dep.service || '';
    document.getElementById('dep-type-' + idx).value = dep.type || 'running';
    document.getElementById('dep-timeout-' + idx).value = formatNsDuration(dep.timeout) || '60s';
  }
}

function removeDependency(idx) {
  const el = document.getElementById('dep-' + idx);
  if (el) el.remove();
}

function loadDependencies(deps) {
  resetDependencies();
  if (deps) {
    for (const d of deps) addDependency(d);
  }
}

function collectDependencies() {
  const deps = [];
  for (let i = 0; i < depCount; i++) {
    const el = document.getElementById('dep-' + i);
    if (!el) continue;
    const svc = document.getElementById('dep-svc-' + i).value.trim();
    if (!svc) continue;
    deps.push({
      service: svc,
      type: document.getElementById('dep-type-' + i).value,
      timeout: parseDurationToNs(document.getElementById('dep-timeout-' + i).value.trim()),
    });
  }
  return deps.length ? deps : undefined;
}
