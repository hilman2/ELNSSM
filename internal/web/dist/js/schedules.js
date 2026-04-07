/* schedules.js - Schedule management tab in service modal */

let scheduleCount = 0;

function resetSchedules() {
  scheduleCount = 0;
  document.getElementById('schedules-list').innerHTML = '';
}

function addSchedule(sched) {
  const idx = scheduleCount++;
  const list = document.getElementById('schedules-list');
  const div = document.createElement('div');
  div.className = 'hc-item';
  div.id = 'sched-' + idx;
  div.innerHTML = `
    <button class="hc-remove" onclick="removeSchedule(${idx})">Remove</button>
    <div class="form-row">
      <div class="form-group"><label>Cron Expression</label>
        <input type="text" id="sched-cron-${idx}" placeholder="0 8 * * MON-FRI">
      </div>
      <div class="form-group"><label>Action</label>
        <select id="sched-action-${idx}">
          <option value="start">Start</option>
          <option value="stop">Stop</option>
          <option value="restart">Restart</option>
        </select>
      </div>
    </div>
    <div class="form-group"><label>Label (optional)</label>
      <input type="text" id="sched-name-${idx}" placeholder="Weekday morning start">
    </div>`;
  list.appendChild(div);
  if (sched) {
    document.getElementById('sched-cron-' + idx).value = sched.cron || '';
    document.getElementById('sched-action-' + idx).value = sched.action || 'start';
    document.getElementById('sched-name-' + idx).value = sched.name || '';
  }
}

function removeSchedule(idx) {
  const el = document.getElementById('sched-' + idx);
  if (el) el.remove();
}

function loadSchedules(schedules) {
  resetSchedules();
  if (schedules) {
    for (const s of schedules) addSchedule(s);
  }
}

function collectSchedules() {
  const schedules = [];
  for (let i = 0; i < scheduleCount; i++) {
    const el = document.getElementById('sched-' + i);
    if (!el) continue;
    const cron = document.getElementById('sched-cron-' + i).value.trim();
    if (!cron) continue;
    schedules.push({
      cron: cron,
      action: document.getElementById('sched-action-' + i).value,
      name: document.getElementById('sched-name-' + i).value.trim() || undefined,
    });
  }
  return schedules.length ? schedules : undefined;
}
