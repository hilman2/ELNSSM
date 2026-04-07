/* notifications.js - Notification settings modal (SMTP, Telegram, ntfy, Webhooks) */

let webhookCount = 0;

function showNotificationsModal() {
  document.getElementById('notifications-modal').style.display = 'block';
  loadNotificationSettings();
}

function hideNotificationsModal() {
  document.getElementById('notifications-modal').style.display = 'none';
}

async function loadNotificationSettings() {
  const res = await api('GET', '/config');
  const d = res.data;
  if (!d) return;

  // SMTP
  const smtp = d.smtp || {};
  document.getElementById('s-smtp-enabled').checked = !!smtp.enabled;
  document.getElementById('s-smtp-host').value = smtp.host || '';
  document.getElementById('s-smtp-port').value = smtp.port || 587;
  document.getElementById('s-smtp-user').value = smtp.username || '';
  document.getElementById('s-smtp-from').value = smtp.from || '';
  document.getElementById('s-smtp-tls').checked = smtp.tls !== false;
  document.getElementById('s-smtp-recipients').value = (smtp.recipients || []).join('\n');

  // Telegram
  const tg = d.telegram || {};
  document.getElementById('s-tg-enabled').checked = !!tg.enabled;
  document.getElementById('s-tg-token').value = '';
  document.getElementById('s-tg-chats').value = (tg.chat_ids || []).join('\n');

  // ntfy
  const ntfy = d.ntfy || {};
  document.getElementById('s-ntfy-enabled').checked = !!ntfy.enabled;
  document.getElementById('s-ntfy-server').value = ntfy.server || 'https://ntfy.sh';
  document.getElementById('s-ntfy-topic').value = ntfy.topic || '';
  document.getElementById('s-ntfy-token').value = '';
  document.getElementById('s-ntfy-priority').value = ntfy.priority || 'default';
  document.getElementById('s-ntfy-tags').value = ntfy.tags || '';
  document.getElementById('s-ntfy-email').value = ntfy.email || '';

  // Cooldown
  document.getElementById('s-cooldown').value = d.notification_cooldown || '5m';

  // Webhooks
  webhookCount = 0;
  document.getElementById('webhook-list').innerHTML = '';
  if (d.webhooks) { for (const wh of d.webhooks) addWebhook(wh); }
}

function addWebhook(wh) {
  const idx = webhookCount++;
  const list = document.getElementById('webhook-list');
  const div = document.createElement('div');
  div.className = 'hc-item';
  div.id = 'wh-' + idx;
  div.innerHTML = `
    <button class="hc-remove" onclick="document.getElementById('wh-${idx}').remove()">Remove</button>
    <div class="form-row">
      <div class="form-group"><label>Name</label><input type="text" id="wh-name-${idx}" placeholder="my-webhook"></div>
      <div class="form-group"><label><input type="checkbox" id="wh-enabled-${idx}" checked> Enabled</label></div>
    </div>
    <div class="form-group"><label>URL</label><input type="text" id="wh-url-${idx}" placeholder="https://hooks.example.com/notify"></div>
    <div class="form-group"><label>Events (comma-separated, empty = all)</label><input type="text" id="wh-events-${idx}" placeholder="service.crashed,health_check.failed"></div>`;
  list.appendChild(div);
  if (wh) {
    document.getElementById('wh-name-' + idx).value = wh.name || '';
    document.getElementById('wh-enabled-' + idx).checked = wh.enabled !== false;
    document.getElementById('wh-url-' + idx).value = wh.url || '';
    document.getElementById('wh-events-' + idx).value = (wh.events || []).join(',');
  }
}

async function saveNotifications() {
  const body = {};

  body.smtp = {
    enabled: document.getElementById('s-smtp-enabled').checked,
    host: document.getElementById('s-smtp-host').value.trim(),
    port: parseInt(document.getElementById('s-smtp-port').value) || 587,
    username: document.getElementById('s-smtp-user').value.trim(),
    password: document.getElementById('s-smtp-pass').value || undefined,
    from: document.getElementById('s-smtp-from').value.trim(),
    tls: document.getElementById('s-smtp-tls').checked,
    recipients: document.getElementById('s-smtp-recipients').value.trim().split('\n').filter(l => l.trim()),
  };

  const tgToken = document.getElementById('s-tg-token').value.trim();
  body.telegram = {
    enabled: document.getElementById('s-tg-enabled').checked,
    bot_token: tgToken || undefined,
    chat_ids: document.getElementById('s-tg-chats').value.trim().split('\n').filter(l => l.trim()),
  };

  const ntfyToken = document.getElementById('s-ntfy-token').value.trim();
  body.ntfy = {
    enabled: document.getElementById('s-ntfy-enabled').checked,
    server: document.getElementById('s-ntfy-server').value.trim() || undefined,
    topic: document.getElementById('s-ntfy-topic').value.trim() || undefined,
    token: ntfyToken || undefined,
    priority: document.getElementById('s-ntfy-priority').value || undefined,
    tags: document.getElementById('s-ntfy-tags').value.trim() || undefined,
    email: document.getElementById('s-ntfy-email').value.trim() || undefined,
  };

  const webhooks = [];
  for (let i = 0; i < webhookCount; i++) {
    if (!document.getElementById('wh-' + i)) continue;
    const url = document.getElementById('wh-url-' + i).value.trim();
    if (!url) continue;
    const evts = document.getElementById('wh-events-' + i).value.trim();
    webhooks.push({
      name: document.getElementById('wh-name-' + i).value.trim() || 'webhook-' + i,
      enabled: document.getElementById('wh-enabled-' + i).checked,
      url,
      method: 'POST',
      events: evts ? evts.split(',').map(e => e.trim()) : [],
    });
  }
  body.webhooks = webhooks;
  body.notification_cooldown = document.getElementById('s-cooldown').value.trim() || '5m';

  await api('PUT', '/config', body);
  hideNotificationsModal();
  alert('Notification settings saved. Changes take effect after Guardian restart.');
}
