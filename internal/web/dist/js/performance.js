/* performance.js - Performance graphs using Canvas */

let perfModalSvcId = null;

function showPerformanceModal(id) {
  perfModalSvcId = id;
  document.getElementById('perf-modal').style.display = 'block';
  document.getElementById('perf-title').textContent = 'Performance: ' + id;
  document.getElementById('perf-range').value = '1h';
  loadPerformanceData();
}

function hidePerformanceModal() {
  document.getElementById('perf-modal').style.display = 'none';
  perfModalSvcId = null;
}

async function loadPerformanceData() {
  if (!perfModalSvcId) return;
  const range = document.getElementById('perf-range').value;
  const res = await api('GET', '/services/' + perfModalSvcId + '/performance?range=' + range + '&points=200');
  const data = res.data;
  if (!data || !data.samples || data.samples.length === 0) {
    const canvas = document.getElementById('perf-canvas');
    const ctx = canvas.getContext('2d');
    canvas.width = canvas.parentElement.offsetWidth;
    canvas.height = 250;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-muted');
    ctx.font = '14px Poppins, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('No performance data available for this time range', canvas.width / 2, 125);
    return;
  }
  drawPerformanceChart(data.samples);
}

function drawPerformanceChart(samples) {
  const canvas = document.getElementById('perf-canvas');
  const wrap = canvas.parentElement;
  canvas.width = wrap.offsetWidth;
  canvas.height = 250;
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  const pad = { top: 20, right: 60, bottom: 30, left: 50 };
  const plotW = w - pad.left - pad.right;
  const plotH = h - pad.top - pad.bottom;

  const style = getComputedStyle(document.documentElement);
  const textColor = style.getPropertyValue('--text-secondary').trim();
  const borderColor = style.getPropertyValue('--border').trim();
  const accentColor = style.getPropertyValue('--accent').trim();
  const successColor = style.getPropertyValue('--success').trim();

  ctx.clearRect(0, 0, w, h);

  // Background grid
  ctx.strokeStyle = borderColor;
  ctx.lineWidth = 0.5;
  for (let i = 0; i <= 4; i++) {
    const y = pad.top + (plotH / 4) * i;
    ctx.beginPath();
    ctx.moveTo(pad.left, y);
    ctx.lineTo(w - pad.right, y);
    ctx.stroke();
  }

  // CPU axis labels (left)
  ctx.fillStyle = textColor;
  ctx.font = '11px Poppins, sans-serif';
  ctx.textAlign = 'right';
  for (let i = 0; i <= 4; i++) {
    const y = pad.top + (plotH / 4) * i;
    ctx.fillText((100 - i * 25) + '%', pad.left - 8, y + 4);
  }

  // Memory axis labels (right)
  const maxMem = Math.max(...samples.map(s => s.memory_bytes || 0));
  ctx.textAlign = 'left';
  for (let i = 0; i <= 4; i++) {
    const y = pad.top + (plotH / 4) * i;
    const memVal = maxMem * (1 - i / 4);
    ctx.fillText(formatBytes(memVal), w - pad.right + 8, y + 4);
  }

  // Time labels
  const times = samples.map(s => new Date(s.timestamp));
  ctx.textAlign = 'center';
  const labelCount = Math.min(6, samples.length);
  for (let i = 0; i < labelCount; i++) {
    const idx = Math.floor((samples.length - 1) * i / (labelCount - 1));
    const x = pad.left + (plotW * idx) / (samples.length - 1);
    const t = times[idx];
    ctx.fillText(t.getHours().toString().padStart(2, '0') + ':' + t.getMinutes().toString().padStart(2, '0'), x, h - 5);
  }

  // Draw CPU line
  ctx.strokeStyle = accentColor;
  ctx.lineWidth = 2;
  ctx.beginPath();
  for (let i = 0; i < samples.length; i++) {
    const x = pad.left + (plotW * i) / (samples.length - 1);
    const y = pad.top + plotH * (1 - samples[i].cpu_percent / 100);
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  }
  ctx.stroke();

  // Draw Memory line
  if (maxMem > 0) {
    ctx.strokeStyle = successColor;
    ctx.lineWidth = 2;
    ctx.beginPath();
    for (let i = 0; i < samples.length; i++) {
      const x = pad.left + (plotW * i) / (samples.length - 1);
      const y = pad.top + plotH * (1 - (samples[i].memory_bytes || 0) / maxMem);
      if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
    }
    ctx.stroke();
  }

  // Axis labels
  ctx.fillStyle = accentColor;
  ctx.font = '12px Poppins, sans-serif';
  ctx.textAlign = 'center';
  ctx.save();
  ctx.translate(12, pad.top + plotH / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText('CPU %', 0, 0);
  ctx.restore();

  ctx.fillStyle = successColor;
  ctx.save();
  ctx.translate(w - 8, pad.top + plotH / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText('Memory', 0, 0);
  ctx.restore();
}
