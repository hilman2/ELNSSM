/* utils.js - Formatters, escape helpers, duration parsers */

function esc(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

function formatDuration(ns) {
  const sec = Math.floor(ns / 1e9);
  if (sec < 60) return sec + 's';
  if (sec < 3600) return Math.floor(sec / 60) + 'm ' + (sec % 60) + 's';
  return Math.floor(sec / 3600) + 'h ' + Math.floor((sec % 3600) / 60) + 'm';
}

function formatNsDuration(ns) {
  if (!ns || ns === 0) return '';
  const s = ns / 1e9;
  if (s < 60) return s + 's';
  if (s < 3600) return (s / 60) + 'm';
  return (s / 3600) + 'h';
}

function parseDurationToNs(str) {
  if (!str) return 0;
  str = str.trim();
  if (!str) return 0;
  let total = 0;
  const re = /(\d+(?:\.\d+)?)(ms|s|m|h)/g;
  let m;
  while ((m = re.exec(str)) !== null) {
    const v = parseFloat(m[1]);
    switch (m[2]) {
      case 'ms': total += v * 1e6; break;
      case 's': total += v * 1e9; break;
      case 'm': total += v * 60e9; break;
      case 'h': total += v * 3600e9; break;
    }
  }
  return total;
}

function formatBytes(b) {
  if (!b || b === 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  const k = 1024;
  const i = Math.floor(Math.log(b) / Math.log(k));
  return (b / Math.pow(k, i)).toFixed(1) + ' ' + u[i];
}

function linkifyUrls(text) {
  const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  return escaped.replace(/(https?:\/\/[^\s<')\]]*[^\s<')\].,:;!?"&])/g, function(match) {
    let url = match.replace(/&(quot|amp|lt|gt|#\d+|#x[0-9a-f]+);?$/gi, '');
    url = url.replace(/[.,;:!?)}\]]+$/, '');
    if (!url) return match;
    return '<a href="' + url + '" target="_blank" rel="noopener">' + url + '</a>' + match.slice(url.length);
  });
}

function formatTimeAgo(dateStr) {
  const d = new Date(dateStr);
  const diff = Math.floor((Date.now() - d.getTime()) / 1000);
  if (diff < 60) return diff + 's ago';
  if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
  if (diff < 86400) return Math.floor(diff / 3600) + 'h ago';
  return Math.floor(diff / 86400) + 'd ago';
}
