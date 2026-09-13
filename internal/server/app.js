(async function () {
  const select = document.getElementById('exercise-select');
  const chart = document.getElementById('chart');
  if (!select || !chart) return;

  select.addEventListener('change', () => load(select.value));
  if (select.options.length > 0) load(select.value);

  async function load(id) {
    try {
      const res = await fetch('/api/exercises/' + encodeURIComponent(id) + '/progress');
      if (!res.ok) throw new Error('HTTP ' + res.status);
      draw(await res.json(), chart);
    } catch (err) {
      chart.innerText = 'failed to load progress: ' + err;
    }
  }
})();

function draw(points, chart) {
  if (!points.length) {
    chart.innerHTML = '<p>No data yet for this exercise.</p>';
    return;
  }
  points = points.slice().sort((a, b) => parseDate(a.date) - parseDate(b.date));

  const W = 920, H = 360, PL = 48, PR = 16, PT = 18, PB = 46;
  const iw = W - PL - PR, ih = H - PT - PB;

  function parseDate(s) {
    const iso = /^\d{4}-\d{2}-\d{2}$/.test(s) ? s + 'T00:00:00' : s;
    return new Date(iso).getTime();
  }

  let minT = Infinity, maxT = -Infinity, minB = Infinity, maxB = -Infinity;
  for (const p of points) {
    minT = Math.min(minT, parseDate(p.date));
    maxT = Math.max(maxT, parseDate(p.date));
    const lo = Math.min(p.start_bpm, p.end_bpm), hi = Math.max(p.start_bpm, p.end_bpm);
    minB = Math.min(minB, lo);
    maxB = Math.max(maxB, hi);
  }
  if (minT === maxT) maxT = minT + 86400000;
  if (minB === maxB) { minB -= 10; maxB += 10; }
  minB = Math.floor((minB - 5) / 10) * 10;
  maxB = Math.ceil((maxB + 5) / 10) * 10;

  const x = t => PL + ((t - minT) / (maxT - minT)) * iw;
  const y = b => PT + ih - ((b - minB) / (maxB - minB)) * ih;
  const longDate = d => d.toISOString().slice(0, 10);
  const tickDate = d => String(d.getUTCMonth() + 1).padStart(2, '0') + '-' +
    String(d.getUTCDate()).padStart(2, '0');

  let s = '<svg width="' + W + '" height="' + H + '" viewBox="0 0 ' + W + ' ' + H +
    '" role="img" font-family="system-ui,sans-serif" font-size="11">';
  s += '<rect width="' + W + '" height="' + H + '" fill="#fafafa"/>';

  const step = Math.max(1, Math.round((maxB - minB) / 5 / 10) * 10);
  for (let b = minB; b <= maxB; b += step) {
    const yy = y(b).toFixed(1);
    s += '<line x1="' + PL + '" y1="' + yy + '" x2="' + (W - PR) + '" y2="' + yy +
      '" stroke="#e2e2e2"/>';
    s += '<text x="' + (PL - 7) + '" y="' + (parseFloat(yy) + 3) + '" text-anchor="end">' + b + '</text>';
  }
  const ticks = 6;
  for (let i = 0; i <= ticks; i++) {
    const t = minT + ((maxT - minT) * i) / ticks;
    const px = x(t).toFixed(1);
    s += '<line x1="' + px + '" y1="' + PT + '" x2="' + px + '" y2="' + (H - PB) +
      '" stroke="' + (i === 0 || i === ticks ? '#ccc' : '#e2e2e2') + '"/>';
    s += '<text x="' + px + '" y="' + (H - PB + 15) + '" text-anchor="middle">' +
      tickDate(new Date(t)) + '</text>';
  }

  const line = key => {
    let d = '';
    for (const p of points) {
      d += (d ? 'L' : 'M') + x(parseDate(p.date)).toFixed(1) + ',' + y(p[key]).toFixed(1) + ' ';
    }
    return d;
  };
  s += '<path d="' + line('start_bpm') + '" fill="none" stroke="#4a90d9" stroke-width="2" stroke-linejoin="round"/>';
  s += '<path d="' + line('end_bpm') + '" fill="none" stroke="#e07b39" stroke-width="2" stroke-linejoin="round"/>';

  for (const p of points) {
    const cx = x(parseDate(p.date)).toFixed(1);
    const cy = y(p.end_bpm).toFixed(1);
    const note = p.notes ? ' (' + p.notes.replace(/"/g, '&quot;') + ')' : '';
    s += '<circle cx="' + cx + '" cy="' + cy + '" r="3" fill="#e07b39">' +
      '<title>' + longDate(new Date(parseDate(p.date))) + ': ' + p.start_bpm + ' -> ' + p.end_bpm +
      ' bpm' + note + '</title></circle>';
  }
  s += '<text x="' + PL + '" y="' + (H - 6) + '" fill="#4a90d9">start bpm</text>';
  s += '<text x="' + (PL + 58) + '" y="' + (H - 6) + '" fill="#e07b39">end bpm</text>';
  s += '<text x="' + PL + '" y="14" font-size="12">' + longDate(new Date(Math.min(minT, maxT))) +
    ' — ' + longDate(new Date(maxT)) + '</text>';
  s += '</svg>';

  chart.innerHTML = s;
  chart.querySelector('svg').setAttribute('aria-label', 'BPM progress over time');
}