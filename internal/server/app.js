(async function () {
  const select = document.getElementById('exercise-select');
  const chart = document.getElementById('chart');
  const filter = document.getElementById('topic-filter');
  if (!select || !chart) return;

  const rows = Array.from(document.querySelectorAll('tr[data-topic]'));
  const opts = Array.from(select.options);

  function applyFilter() {
    if (!opts.length) return;
    const t = filter ? filter.value : '';
    let visible = 0;
    for (const o of opts) {
      const show = !t || o.dataset.topic === t;
      o.style.display = show ? '' : 'none';
      if (show) visible++;
    }
    for (const tr of rows) tr.style.display = (!t || tr.dataset.topic === t) ? '' : 'none';

    if (visible === 0) {
      chart.innerHTML = '<p>No exercises for this topic.</p>';
      return;
    }
    const sel = select.selectedOptions[0];
    if (!sel || sel.style.display === 'none') {
      for (const o of opts) {
        if (o.style.display !== 'none') { select.value = o.value; break; }
      }
    }
    load(select.value);
  }

  if (filter) filter.addEventListener('change', applyFilter);
  select.addEventListener('change', () => load(select.value));
  applyFilter();

  const logForm = document.getElementById('log-form');
  if (logForm) {
    const lf = id => logForm.querySelector(id);
    const status = document.getElementById('log-status');
    logForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const headers = { 'Content-Type': 'application/json' };
      const cfg = { exercises_per_round: 1, duration_sec: 1, rest_sec: 0, break_sec: 0 };
      const entry = {
        exercise_id: lf('#log-exercise').value,
        round: 1, sequence: 1,
        start_bpm: parseInt(lf('#log-start').value || '0', 10),
        end_bpm: parseInt(lf('#log-end').value || '0', 10),
        notes: lf('#log-notes').value.trim(),
        started_at: new Date().toISOString(),
        finished_at: new Date().toISOString()
      };
      try {
        const res = await fetch('/api/sessions', {
          method: 'POST', headers,
          body: JSON.stringify({ exercise_ids: [entry.exercise_id], config: cfg })
        });
        if (!res.ok) throw new Error('create session HTTP ' + res.status);
        const sess = await res.json();
        const er = await fetch('/api/sessions/' + sess.id + '/entries', {
          method: 'POST', headers, body: JSON.stringify(entry)
        });
        if (!er.ok) throw new Error('add entry HTTP ' + er.status);
        await fetch('/api/sessions/' + sess.id + '/end', { method: 'POST' });
        lf('#log-start').value = ''; lf('#log-end').value = ''; lf('#log-notes').value = '';
        status.style.color = '#7ee787';
        status.innerText = 'logged. refreshing…';
        setTimeout(() => location.reload(), 500);
      } catch (err) {
        status.style.color = '#ff6b6b';
        status.innerText = 'failed: ' + err;
      }
    });
  }

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

const DARK = {
  bg: '#0b0f14',
  grid: '#1c2733',
  axis: '#31404f',
  text: '#8290a0',
  start: '#5aa2e8',
  end: '#f0875a'
};

function draw(points, chart) {
  if (!points.length) {
    chart.innerHTML = '<p>No data yet for this exercise.</p>';
    return;
  }
  points = points.slice().sort((a, b) => parseDate(a.date) - parseDate(b.date));

  let bestB = 0;
  for (const p of points) {
    p.isPR = p.end_bpm > bestB;
    if (p.end_bpm > bestB) bestB = p.end_bpm;
  }

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
  s += '<rect width="' + W + '" height="' + H + '" fill="' + DARK.bg + '"/>';

  const step = Math.max(1, Math.round((maxB - minB) / 5 / 10) * 10);
  for (let b = minB; b <= maxB; b += step) {
    const yy = y(b).toFixed(1);
    s += '<line x1="' + PL + '" y1="' + yy + '" x2="' + (W - PR) + '" y2="' + yy +
      '" stroke="' + DARK.grid + '"/>';
    s += '<text x="' + (PL - 7) + '" y="' + (parseFloat(yy) + 3) + '" text-anchor="end" fill="' +
      DARK.text + '">' + b + '</text>';
  }
  const ticks = 6;
  for (let i = 0; i <= ticks; i++) {
    const t = minT + ((maxT - minT) * i) / ticks;
    const px = x(t).toFixed(1);
    s += '<line x1="' + px + '" y1="' + PT + '" x2="' + px + '" y2="' + (H - PB) +
      '" stroke="' + (i === 0 || i === ticks ? DARK.axis : DARK.grid) + '"/>';
    s += '<text x="' + px + '" y="' + (H - PB + 15) + '" text-anchor="middle" fill="' +
      DARK.text + '">' + tickDate(new Date(t)) + '</text>';
  }

  const line = key => {
    let d = '';
    for (const p of points) {
      d += (d ? 'L' : 'M') + x(parseDate(p.date)).toFixed(1) + ',' + y(p[key]).toFixed(1) + ' ';
    }
    return d;
  };
  s += '<path d="' + line('start_bpm') + '" fill="none" stroke="' + DARK.start +
    '" stroke-width="2" stroke-linejoin="round"/>';
  s += '<path d="' + line('end_bpm') + '" fill="none" stroke="' + DARK.end +
    '" stroke-width="2" stroke-linejoin="round"/>';

  for (const p of points) {
    const cx = x(parseDate(p.date)).toFixed(1);
    const cy = y(p.end_bpm).toFixed(1);
    const note = p.notes ? ' (' + p.notes.replace(/"/g, '&quot;') + ')' : '';
    s += '<circle cx="' + cx + '" cy="' + cy + '" r="3" fill="' + DARK.end + '">' +
      '<title>' + longDate(new Date(parseDate(p.date))) + ': ' + p.start_bpm + ' -> ' + p.end_bpm +
      ' bpm' + note + '</title></circle>';
    if (p.isPR) {
      s += '<text x="' + cx + '" y="' + (parseFloat(cy) - 8) +
        '" text-anchor="middle" fill="#ffd45e" font-size="13">★</text>';
    }
  }
  s += '<text x="' + PL + '" y="' + (H - 6) + '" fill="' + DARK.start + '">start bpm</text>';
  s += '<text x="' + (PL + 58) + '" y="' + (H - 6) + '" fill="' + DARK.end + '">end bpm</text>';
  if (bestB > 0) {
    s += '<text x="' + (PL + 116) + '" y="' + (H - 6) + '" fill="#ffd45e">best ' + bestB + '</text>';
  }
  s += '<text x="' + PL + '" y="14" font-size="12" fill="' + DARK.text + '">' +
    longDate(new Date(Math.min(minT, maxT))) + ' — ' + longDate(new Date(maxT)) + '</text>';
  s += '</svg>';

  chart.innerHTML = s;
  chart.querySelector('svg').setAttribute('aria-label', 'BPM progress over time');
}