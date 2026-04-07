/* hooks.js - Lifecycle hooks tab in service modal */

function resetHooks() {
  ['pre_start', 'post_start', 'pre_stop', 'post_stop'].forEach(hook => {
    document.getElementById('f-hook-' + hook + '-cmd').value = '';
    document.getElementById('f-hook-' + hook + '-args').value = '';
    document.getElementById('f-hook-' + hook + '-script').value = '';
    document.getElementById('f-hook-' + hook + '-timeout').value = '30s';
    document.getElementById('f-hook-' + hook + '-onfail').value = 'continue';
  });
}

function loadHooks(hooks) {
  if (!hooks) return;
  const map = {
    pre_start: hooks.pre_start,
    post_start: hooks.post_start,
    pre_stop: hooks.pre_stop,
    post_stop: hooks.post_stop
  };
  for (const [key, h] of Object.entries(map)) {
    if (!h) continue;
    document.getElementById('f-hook-' + key + '-cmd').value = h.command || '';
    document.getElementById('f-hook-' + key + '-args').value = (h.args || []).join(' ');
    document.getElementById('f-hook-' + key + '-script').value = h.script || '';
    document.getElementById('f-hook-' + key + '-timeout').value = formatNsDuration(h.timeout) || '30s';
    document.getElementById('f-hook-' + key + '-onfail').value = h.on_failure || 'continue';
  }
}

function collectHooks() {
  const hooks = {};
  ['pre_start', 'post_start', 'pre_stop', 'post_stop'].forEach(key => {
    const cmd = document.getElementById('f-hook-' + key + '-cmd').value.trim();
    const argsStr = document.getElementById('f-hook-' + key + '-args').value.trim();
    const script = document.getElementById('f-hook-' + key + '-script').value.trim();
    if (!cmd && !script) return;
    hooks[key] = {
      command: cmd || undefined,
      args: argsStr ? argsStr.split(' ') : undefined,
      script: script || undefined,
      timeout: parseDurationToNs(document.getElementById('f-hook-' + key + '-timeout').value.trim()),
      on_failure: document.getElementById('f-hook-' + key + '-onfail').value,
    };
  });
  return Object.keys(hooks).length ? hooks : undefined;
}
