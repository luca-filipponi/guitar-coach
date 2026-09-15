(function () {
  var sessionEl = document.querySelector('[data-session]');
  if (!sessionEl) return;
  var sessionId = sessionEl.dataset.session;

  function flash(entry) {
    entry.classList.add('saving');
    setTimeout(function () { entry.classList.remove('saving'); }, 1400);
  }

  document.querySelectorAll('.entry [data-field]').forEach(function (field) {
    field.addEventListener('change', function () {
      var idx = field.dataset.idx;
      var body = {};
      body[field.dataset.field] = field.type === 'number'
        ? (field.value === '' ? 0 : Number(field.value))
        : field.value;
      fetch('/api/sessions/' + encodeURIComponent(sessionId) + '/entries/' + encodeURIComponent(idx), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      }).then(function (res) {
        if (res.ok) {
          flash(field.closest('.entry'));
        } else {
          alert('save failed (HTTP ' + res.status + ')');
        }
      });
    });
  });
})();

(async function () {
  const select = document.getElementById('exercise-select');
  const chart = document.getElementById('chart');
  const filter = document.getElementById('topic-filter');
  if (!select || !chart) return;

  if (filter) filter.addEventListener('change', () => {
    location.href = filter.value ? '/?topic=' + encodeURIComponent(filter.value) : '/';
  });
  select.addEventListener('change', () => load(select.value));

  document.querySelectorAll('tr[data-href]').forEach(tr => {
    tr.addEventListener('click', e => {
      if (e.target.closest('a')) return;
      location.href = tr.dataset.href;
    });
  });

  load(select.value);

  async function load(id) {
    try {
      const res = await fetch('/api/exercises/' + encodeURIComponent(id) + '/progress');
      if (!res.ok) throw new Error('HTTP ' + res.status);
      render(await res.json(), chart);
    } catch (err) {
      chart.innerHTML = '<p>failed to load progress: ' + err + '</p>';
    }
  }

  function parseDate(s) {
    const iso = /^\d{4}-\d{2}-\d{2}$/.test(s) ? s + 'T00:00:00' : s;
    return new Date(iso).getTime() / 1000;
  }

  function render(points, container) {
    if (chart._u) { chart._u.destroy(); chart._u = null; }
    container.innerHTML = '';
    if (!points.length) {
      container.innerHTML = '<p>No data yet for this exercise.</p>';
      return;
    }
    points = points.slice().sort((a, b) => parseDate(a.date) - parseDate(b.date));

    let best = 0;
    const pr = points.map(p => { const is = p.end_bpm > best; if (p.end_bpm > best) best = p.end_bpm; return is; });
    const data = [[], [], []];
    for (let i = 0; i < points.length; i++) {
      const p = points[i];
      data[0].push(parseDate(p.date));
      data[1].push(p.start_bpm > 0 ? p.start_bpm : null);
      data[2].push(p.end_bpm > 0 ? p.end_bpm : null);
    }

    const cap = document.createElement('p');
    cap.className = 'hint';
    cap.textContent = best > 0
      ? 'best ' + best + ' bpm — gold points are personal records'
      : 'hover the chart to inspect values';
    container.appendChild(cap);

    const target = document.createElement('div');
    container.appendChild(target);

    const palette = { start: '#5aa2e8', end: '#f0875a', gold: '#ffd45e' };

    const base = container;
    base.style.position = 'relative';
    const tip = document.createElement('div');
    tip.className = 'chart-note';
    base.appendChild(tip);

    const fmtDate = ts => {
      const d = new Date(ts * 1000);
      return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' +
        d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
    };

    chart._u = new uPlot({
      width: target.clientWidth || 760,
      height: 320,
      padding: [14, 14, 8, 8],
      scales: { x: { time: true }, y: { auto: true } },
      axes: [
        { stroke: '#9fb2c6', grid: { stroke: 'rgba(120,145,175,.14)' }, ticks: { stroke: '#31404f' } },
        { stroke: '#9fb2c6', grid: { stroke: 'rgba(120,145,175,.14)' }, ticks: { stroke: '#31404f' }, label: 'BPM' }
      ],
      hooks: {
        setCursor: [u => {
          const idx = u.cursor.idx;
          if (idx == null || idx < 0 || !points[idx]) {
            tip.style.display = 'none';
            return;
          }
          const p = points[idx];
          tip.innerHTML = '<b>' + fmtDate(data[0][idx]) + '</b> · ' +
            (p.start_bpm > 0 ? p.start_bpm : '—') + ' → ' +
            (p.end_bpm > 0 ? p.end_bpm + ' bpm' : '—') +
            (p.notes ? '<br><span>' + escapeHtml(p.notes) + '</span>' : '');
          tip.style.display = 'block';
          const overRect = u.over.getBoundingClientRect();
          const baseRect = base.getBoundingClientRect();
          const gap = 12;
          let l = overRect.left - baseRect.left + u.cursor.left + gap;
          let t = overRect.top - baseRect.top + u.cursor.top + gap;
          tip.style.left = l + 'px';
          tip.style.top = t + 'px';
          const maxL = base.clientWidth - tip.offsetWidth - 6;
          if (l > maxL) tip.style.left = Math.max(0, overRect.left - baseRect.left + u.cursor.left - tip.offsetWidth - gap) + 'px';
          const maxT = base.clientHeight - tip.offsetHeight - 6;
          if (t > maxT) tip.style.top = Math.max(0, overRect.top - baseRect.top + u.cursor.top - tip.offsetHeight - gap) + 'px';
        }]
      },
      series: [
        {
          label: 'date',
          value: (u, ts) => {
            if (ts === null || ts === undefined) return '-';
            return fmtDate(ts);
          }
        },
        {
          label: 'start bpm', stroke: palette.start, width: 2, spanGaps: true,
          value: raw => raw === null ? '-' : raw + ' bpm'
        },
        {
          label: 'end bpm', stroke: palette.end, width: 2, spanGaps: true,
          value: raw => raw === null ? '-' : raw + ' bpm',
          points: {
            show: true,
            size: (u, i) => pr[i] ? 9 : 4,
            stroke: (u, i) => pr[i] ? palette.gold : palette.end,
            fill: (u, i) => pr[i] ? 'rgba(255,212,94,.25)' : '#0b0f14'
          }
        }
      ]
    }, data, target);
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }
})();