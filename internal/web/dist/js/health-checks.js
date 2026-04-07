/* health-checks.js - Health check form management in service modal */

let healthCheckCount = 0;

function resetHealthChecks() {
  healthCheckCount = 0;
  document.getElementById('health-checks-list').innerHTML = '';
}

function addHealthCheck(hc) {
  const idx = healthCheckCount++;
  const list = document.getElementById('health-checks-list');
  const div = document.createElement('div');
  div.className = 'hc-item';
  div.id = 'hc-' + idx;
  div.innerHTML = `
    <button class="hc-remove" onclick="removeHealthCheck(${idx})">Remove</button>
    <div class="form-row">
      <div class="form-group"><label>Type</label>
        <select id="hc-type-${idx}" onchange="updateHcFields(${idx})"><option value="http">HTTP</option><option value="tcp">TCP</option><option value="script">Script</option></select>
      </div>
      <div class="form-group" id="hc-target-group-${idx}"><label id="hc-target-label-${idx}">URL</label><input type="text" id="hc-target-${idx}" placeholder="http://localhost:3000/health"></div>
    </div>
    <div id="hc-http-fields-${idx}">
      <div class="form-row">
        <div class="form-group"><label>Method</label><input type="text" id="hc-method-${idx}" placeholder="GET"></div>
        <div class="form-group"><label>Expected Status</label><input type="number" id="hc-status-${idx}" placeholder="200"></div>
      </div>
      <div class="form-group"><label>Expected Body (substring)</label><input type="text" id="hc-body-${idx}"></div>
    </div>
    <div id="hc-tcp-fields-${idx}" style="display:none">
      <div class="form-row">
        <div class="form-group"><label>Send Data</label><input type="text" id="hc-send-${idx}" placeholder="PING (optional)"></div>
        <div class="form-group"><label>Expected Response</label><input type="text" id="hc-expect-resp-${idx}" placeholder="PONG (optional)"></div>
      </div>
    </div>
    <div id="hc-script-fields-${idx}" style="display:none">
      <div class="form-group"><label>Script Body (PowerShell/Batch auto-detected)</label>
        <textarea id="hc-script-body-${idx}" style="font-family:monospace;min-height:100px" placeholder="# PowerShell example:&#10;$r = Invoke-WebRequest -Uri http://localhost:3000/health -UseBasicParsing&#10;if ($r.StatusCode -ne 200) { exit 1 }&#10;exit 0"></textarea>
      </div>
    </div>
    <div class="form-row">
      <div class="form-group"><label>Interval</label><input type="text" id="hc-interval-${idx}" value="30s"></div>
      <div class="form-group"><label>Timeout</label><input type="text" id="hc-timeout-${idx}" value="10s"></div>
    </div>
    <div class="form-row">
      <div class="form-group"><label>Retries</label><input type="number" id="hc-retries-${idx}" value="3" min="0"></div>
      <div class="form-group"><label>Start Delay</label><input type="text" id="hc-delay-${idx}" value="15s"></div>
    </div>`;
  list.appendChild(div);
  if (hc) {
    document.getElementById('hc-type-' + idx).value = hc.type || 'http';
    document.getElementById('hc-target-' + idx).value = hc.target || '';
    document.getElementById('hc-method-' + idx).value = hc.method || '';
    document.getElementById('hc-status-' + idx).value = hc.expect_status || '';
    document.getElementById('hc-body-' + idx).value = hc.expect_body || '';
    document.getElementById('hc-send-' + idx).value = hc.send || '';
    document.getElementById('hc-expect-resp-' + idx).value = hc.expect_resp || '';
    document.getElementById('hc-script-body-' + idx).value = hc.script_body || '';
    document.getElementById('hc-interval-' + idx).value = formatNsDuration(hc.interval) || '30s';
    document.getElementById('hc-timeout-' + idx).value = formatNsDuration(hc.timeout) || '10s';
    document.getElementById('hc-retries-' + idx).value = hc.retries || 3;
    document.getElementById('hc-delay-' + idx).value = formatNsDuration(hc.start_delay) || '15s';
    updateHcFields(idx);
  }
}

function updateHcFields(idx) {
  const t = document.getElementById('hc-type-' + idx).value;
  document.getElementById('hc-http-fields-' + idx).style.display = t === 'http' ? '' : 'none';
  document.getElementById('hc-tcp-fields-' + idx).style.display = t === 'tcp' ? '' : 'none';
  document.getElementById('hc-script-fields-' + idx).style.display = t === 'script' ? '' : 'none';
  const lbl = document.getElementById('hc-target-label-' + idx);
  const inp = document.getElementById('hc-target-' + idx);
  if (t === 'http') { lbl.textContent = 'URL'; inp.placeholder = 'http://localhost:3000/health'; }
  else if (t === 'tcp') { lbl.textContent = 'Host:Port'; inp.placeholder = 'localhost:3306'; }
  else { lbl.textContent = 'Command (optional if Script Body set)'; inp.placeholder = 'curl -f http://localhost:3000/health'; }
}

function removeHealthCheck(idx) {
  const el = document.getElementById('hc-' + idx);
  if (el) el.remove();
}

function collectHealthChecks() {
  const checks = [];
  for (let i = 0; i < healthCheckCount; i++) {
    const el = document.getElementById('hc-' + i);
    if (!el) continue;
    const t = document.getElementById('hc-type-' + i).value;
    const target = document.getElementById('hc-target-' + i).value.trim();
    const sb = document.getElementById('hc-script-body-' + i).value.trim();
    if (t !== 'script' && !target) continue;
    if (t === 'script' && !target && !sb) continue;
    checks.push({
      type: t, target: target || undefined,
      method: t === 'http' ? (document.getElementById('hc-method-' + i).value.trim() || undefined) : undefined,
      expect_status: t === 'http' ? (parseInt(document.getElementById('hc-status-' + i).value) || undefined) : undefined,
      expect_body: t === 'http' ? (document.getElementById('hc-body-' + i).value.trim() || undefined) : undefined,
      send: t === 'tcp' ? (document.getElementById('hc-send-' + i).value.trim() || undefined) : undefined,
      expect_resp: t === 'tcp' ? (document.getElementById('hc-expect-resp-' + i).value.trim() || undefined) : undefined,
      script_body: t === 'script' ? (sb || undefined) : undefined,
      interval: parseDurationToNs(document.getElementById('hc-interval-' + i).value.trim()),
      timeout: parseDurationToNs(document.getElementById('hc-timeout-' + i).value.trim()),
      retries: parseInt(document.getElementById('hc-retries-' + i).value) || 3,
      start_delay: parseDurationToNs(document.getElementById('hc-delay-' + i).value.trim()),
    });
  }
  return checks;
}
