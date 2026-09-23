// ─── SVG helpers (ported from Go) ────────────────────────────────────────────

function smoothPath(pts) {
  if (!pts.length) return '';
  let d = 'M' + pts[0].x.toFixed(2) + ' ' + pts[0].y.toFixed(2);
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[Math.max(i - 1, 0)];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[Math.min(i + 2, pts.length - 1)];
    const c1x = p1.x + (p2.x - p0.x) / 6;
    const c1y = p1.y + (p2.y - p0.y) / 6;
    const c2x = p2.x - (p3.x - p1.x) / 6;
    const c2y = p2.y - (p3.y - p1.y) / 6;
    d += ' C' + c1x.toFixed(2) + ' ' + c1y.toFixed(2) + ' ' +
         c2x.toFixed(2) + ' ' + c2y.toFixed(2) + ' ' +
         p2.x.toFixed(2) + ' ' + p2.y.toFixed(2);
  }
  return d;
}

function fillPath(pts, base) {
  if (!pts.length) return '';
  return smoothPath(pts) + ' L' + pts[pts.length - 1].x.toFixed(2) + ' ' + base.toFixed(2) +
         ' L' + pts[0].x.toFixed(2) + ' ' + base.toFixed(2) + ' Z';
}

let _idCounter = 0;
function nextId() { return ++_idCounter; }

function chartPoints(progress) {
  const out = [];
  for (const p of progress) {
    if (p.end_bpm <= 0) continue;
    out.push({ t: parseDate(p.date), start: p.start_bpm, end: p.end_bpm });
  }
  out.sort((a, b) => a.t - b.t);
  return out;
}

function parseDate(s) {
  const iso = /^\d{4}-\d{2}-\d{2}$/.test(s) ? s + 'T00:00:00' : s;
  return new Date(iso).getTime() / 1000;
}

function fmtDate(ts) {
  const d = new Date(ts * 1000);
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' +
         d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

// ─── UI helpers ─────────────────────────────────────────────────────────────

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function friendlyDay(rfc) {
  if (!rfc) return 'n/a';
  const t = new Date(rfc);
  const now = new Date();
  const diffMs = now - t;
  const days = Math.floor(diffMs / 86400000);
  const time = t.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  if (days === 0) return t.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' + time;
  if (days === 1) return 'yesterday ' + time;
  return t.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }) + ' ' + time;
}

function formatDuration(secs) {
  if (secs <= 0) return '—';
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (h === 0) return m + ' min';
  if (m === 0) return h + 'h';
  return h + 'h ' + m + 'm';
}

function fmtMinutes(min) {
  if (min < 60) return min + ' min';
  const h = Math.floor(min / 60);
  const m = min % 60;
  if (m === 0) return h + 'h';
  return h + 'h ' + (m < 10 ? '0' : '') + m + 'm';
}

function formatDelta(d) {
  if (d >= 0) return '+' + d + ' bpm all time';
  return d + ' bpm all time';
}

function deltaColor(d) {
  if (d > 0) return '#3ddc84';
  if (d < 0) return '#e8996a';
  return '#93a3bb';
}

function deltaBadge(progress) {
  const cp = chartPoints(progress);
  if (cp.length < 2) return '<span class="delta new">new</span>';
  const d = cp[cp.length - 1].end - cp[0].end;
  if (d > 0) return '<span class="delta up">\u25B2 +' + d + ' bpm</span>';
  if (d < 0) return '<span class="delta down">\u25BC ' + d + ' bpm</span>';
  return '<span class="delta flat">\u00B10 bpm</span>';
}

// ─── SVG renderers ──────────────────────────────────────────────────────────

function renderSparkline(progress) {
  const cp = chartPoints(progress);
  if (!cp.length) return '<span class="hint">no data</span>';
  const W = 110, H = 26;
  let lo = cp[0].end, hi = cp[0].end;
  for (const p of cp) {
    if (p.end < lo) lo = p.end;
    if (p.end > hi) hi = p.end;
  }
  if (lo === hi) { lo--; hi++; }
  const x = i => 2 + i / (cp.length - 1) * (W - 4);
  const y = v => 3 + (hi - v) / (hi - lo) * (H - 6);
  const pts = cp.map((p, i) => ({ x: x(i), y: y(p.end) }));
  const gid = 'sp' + nextId();
  return '<svg class="spark" viewBox="0 0 110 26" role="img" aria-label="trend" xmlns="http://www.w3.org/2000/svg">' +
    '<defs><linearGradient id="' + gid + '" x1="0" y1="0" x2="0" y2="1">' +
    '<stop offset="0" stop-color="#eba07e" stop-opacity=".55"/>' +
    '<stop offset="1" stop-color="#eba07e" stop-opacity=".02"/>' +
    '</linearGradient></defs>' +
    '<path d="' + fillPath(pts, H - 3) + '" fill="url(#' + gid + ')"/>' +
    '<path d="' + smoothPath(pts) + '" fill="none" stroke="#eba07e" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>' +
    '</svg>';
}

function renderHeatmap(sessions) {
  const dayMins = {};
  let any = false;
  for (const s of sessions) {
    const d = sessionDuration(s);
    if (d <= 0) continue;
    const key = dateKey(new Date(s.started_at));
    dayMins[key] = (dayMins[key] || 0) + Math.round(d / 60);
    any = true;
  }
  if (!any) return '';

  const weeks = 14;
  const cell = 11, gap = 3;
  const stride = cell + gap;
  const LX = 30; // left gutter for weekday labels
  const TY = 12; // top gutter for month labels
  const W = LX + weeks * stride - gap;
  const H = TY + 7 * stride;
  const totalMin = Object.values(dayMins).reduce((a, b) => a + b, 0);

  const end = new Date();
  const endDay = new Date(end.getFullYear(), end.getMonth(), end.getDate() + (6 - end.getDay())); // Saturday of current week
  const startDay = new Date(endDay);
  startDay.setDate(startDay.getDate() - (weeks * 7 - 1));

  const lvl = m => m <= 0 ? 0 : m < 15 ? 1 : m < 30 ? 2 : m < 60 ? 3 : 4;
  const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  const fmtMonth = d => d.toLocaleDateString(undefined, { month: 'short' }).replace('.', '');
  const dayX = w => LX + w * stride;
  const dayY = d => TY + d * stride;

  let svg = '<svg class="heat" viewBox="0 0 ' + W + ' ' + (H + 18) + '" role="img" aria-label="practice minutes per day" xmlns="http://www.w3.org/2000/svg">';

  // Month labels in the top gutter, above the column where a new month begins
  // (GitHub-style). One label per new month, never overlapping the grid.
  let prevMonth = null;
  for (let w = 0; w < weeks; w++) {
    const weekStart = new Date(startDay);
    weekStart.setDate(weekStart.getDate() + w * 7);
    const first1st = new Date(weekStart);
    first1st.setDate(1);
    if (w === 0 || weekStart.getMonth() !== prevMonth) {
      if (first1st >= startDay) {
        svg += '<text x="' + dayX(w) + '" y="' + (TY - 2) + '" class="hm-ref">' + fmtMonth(weekStart) + '</text>';
      } else if (w === 0) {
        svg += '<text x="' + dayX(0) + '" y="' + (TY - 2) + '" class="hm-ref">' + fmtMonth(weekStart) + '</text>';
      }
    }
    prevMonth = weekStart.getMonth();
  }

  // Day-of-week labels on the left border (Mon / Wed / Fri).
  for (const d of [1, 3, 5]) {
    svg += '<text x="' + (LX - 6) + '" y="' + (dayY(d) + cell / 2 + 3) + '" text-anchor="end" class="hm-ref">' + days[d] + '</text>';
  }

  for (let w = 0; w < weeks; w++) {
    for (let d = 0; d < 7; d++) {
      const day = new Date(startDay);
      day.setDate(day.getDate() + w * 7 + d);
      if (day > end) continue;
      const key = dateKey(day);
      const m = dayMins[key] || 0;
      const lv = lvl(m);
      let title = '';
      if (m > 0) {
        title = ' title="' + days[day.getDay()] + ' ' + day.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' — ' + m + ' min"';
      }
      svg += '<rect x="' + dayX(w) + '" y="' + dayY(d) + '" width="' + cell + '" height="' + cell + '" rx="2" class="hm hm' + lv + '"' + title + '/>';
    }
  }
  svg += '</svg>';
  svg += '<p class="hm-total">' + totalMin + ' min practiced in the last ' + (weeks * 7) + ' days</p>';
  return svg;
}

// dateKey renders a local YYYY-MM-DD so day bucketing matches the wall clock,
// not UTC (which can shift a day for evening practice).
function dateKey(d) {
  return d.getFullYear() + '-' +
    String(d.getMonth() + 1).padStart(2, '0') + '-' +
    String(d.getDate()).padStart(2, '0');
}

// ─── Stats ──────────────────────────────────────────────────────────────────

function computeStats(sessions, exercises) {
  const nameByID = {};
  for (const ex of exercises) nameByID[ex.id] = ex.name;
  const st = { sessions: sessions.length, sets: 0, minutes: 0, avgEnd: 0, bestBPM: 0, bestName: '', streak: 0 };
  const daySet = new Set();
  let totalEnd = 0, endCount = 0;
  for (const s of sessions) {
    daySet.add(new Date(s.started_at).toISOString().slice(0, 10));
    for (const e of s.entries) {
      st.sets++;
      const start = new Date(e.started_at);
      const finish = new Date(e.finished_at);
      if (finish > start) st.minutes += Math.floor((finish - start) / 60000);
      if (e.end_bpm > 0) {
        totalEnd += e.end_bpm;
        endCount++;
        if (e.end_bpm > st.bestBPM) { st.bestBPM = e.end_bpm; st.bestName = nameByID[e.exercise_id] || ''; }
      }
    }
  }
  if (endCount > 0) st.avgEnd = Math.round(totalEnd / endCount);

  let cursor = new Date();
  if (!daySet.has(cursor.toISOString().slice(0, 10))) {
    cursor.setDate(cursor.getDate() - 1);
  }
  while (daySet.has(cursor.toISOString().slice(0, 10))) {
    st.streak++;
    cursor.setDate(cursor.getDate() - 1);
  }
  return st;
}

function renderStats(st) {
  const card = (v, label) => '<div class="stat"><b>' + v + '</b><span>' + label + '</span></div>';
  let html = '<div class="stats">';
  html += card(String(st.sessions), 'sessions');
  html += card(String(st.sets), 'sets logged');
  html += card(fmtMinutes(st.minutes), 'min practiced');
  html += card(st.avgEnd > 0 ? st.avgEnd + ' bpm' : '—', 'avg end bpm');
  html += card(st.bestBPM > 0 ? st.bestBPM + ' bpm' : '—', st.bestBPM > 0 ? 'best: ' + escapeHtml(st.bestName) : 'best');
  html += card(String(st.streak), 'day streak');
  html += '</div>';
  return html;
}

function sessionDuration(s) {
  let end = new Date(s.ended_at || 0);
  for (const e of (s.entries || [])) {
    const t = new Date(e.finished_at);
    if (t > end) end = t;
  }
  const start = new Date(s.started_at);
  return end > start ? Math.floor((end - start) / 1000) : 0;
}

// ─── API ────────────────────────────────────────────────────────────────────

async function api(path) {
  const res = await fetch('/api/' + path);
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

// ─── Router ─────────────────────────────────────────────────────────────────

function navigate(path) {
  history.pushState(null, '', path);
  setActiveNav();
  route();
}

async function route() {
  const path = location.pathname;
  const app = document.getElementById('app');
  setActiveNav();

  if (path === '/' || path === '') {
    await renderProgress(app);
  } else if (path === '/exercises') {
    await renderExercises(app, new URLSearchParams(location.search).get('topic') || '');
  } else if (path === '/sessions') {
    await renderSessions(app);
  } else if (path === '/calendar') {
    await renderCalendar(app);
  } else {
    const exMatch = path.match(/^\/exercises\/(.+)$/);
    if (exMatch) { await renderExerciseHistory(app, exMatch[1]); return; }
    const sessMatch = path.match(/^\/sessions\/(.+)$/);
    if (sessMatch) { await renderSessionDetail(app, sessMatch[1]); return; }
    app.innerHTML = '<div class="empty"><h1>Page not found</h1><p><a href="/">Back to dashboard</a></p></div>';
  }
  window.scrollTo(0, 0);
}

function setActiveNav() {
  const path = location.pathname;
  const topic = new URLSearchParams(location.search).get('topic') || '';
  let key = path;
  if (key !== '/' && key !== '/exercises' && key !== '/sessions' && key !== '/calendar') {
    key = key.startsWith('/exercises') ? '/exercises' : key.startsWith('/sessions') ? '/sessions' : '/';
  }
  document.querySelectorAll('.sidebar a').forEach(a => {
    const sub = a.hasAttribute('data-topic');
    let on;
    if (sub) {
      const t = a.getAttribute('data-topic') || '';
      on = key === '/exercises' && (t === 'all' ? topic === '' : t === topic);
    } else {
      on = a.getAttribute('href') === key;
    }
    a.classList.toggle('active', on);
  });
  const group = document.getElementById('navg-exercises');
  if (group) group.classList.toggle('open', key === '/exercises');
}

// ─── Progress view ──────────────────────────────────────────────────────────

function exerciseCardHTML(ex, pts) {
  let inner = '<div class="exhead"><span class="exname">' + escapeHtml(ex.name) + '</span>';
  if (ex.topic) inner += '<span class="pill">' + escapeHtml(ex.topic) + '</span>';
  if (ex.description) inner += '<span class="desc" title="' + escapeHtml(ex.description) + '">' + escapeHtml(ex.description) + '</span>';
  inner += '</div>' + renderSparkline(pts);
  inner += '<div class="exfoot">' + deltaBadge(pts) + '<span class="setcount">' + pts.length + ' sets</span></div>';
  return '<a class="excard" href="/exercises/' + escapeHtml(ex.id) + '" data-nav>' + inner + '</a>';
}

// Group exercises by topic so long lists read as a full catalog (untagged last).
function groupByTopic(exercises) {
  const topicSet = new Set();
  const byTopic = new Map();
  const untagged = [];
  for (const ex of exercises) {
    if (!ex.topic) { untagged.push(ex); continue; }
    topicSet.add(ex.topic);
    if (!byTopic.has(ex.topic)) byTopic.set(ex.topic, []);
    byTopic.get(ex.topic).push(ex);
  }
  return { topics: [...topicSet].sort(), byTopic, untagged };
}

function topicFilterHTML(topics) {
  let html = '<div class="chart-controls" style="margin-bottom:.9rem"><label>Topic: <select id="topic-filter"><option value="">All topics</option>';
  for (const t of topics) html += '<option value="' + escapeHtml(t) + '">' + escapeHtml(t) + '</option>';
  html += '</select></label></div>';
  return html;
}

// Renders one topic group (or all) into gridHost using the passed builder.
// builder(topicName, list) returns the HTML for a single group's list of exercises.
function topicGroupsHTML(topics, byTopic, untagged, topic, builder) {
  let h = '';
  if (!topic) {
    for (const t of topics) {
      const list = byTopic.get(t);
      if (!list || !list.length) continue;
      h += '<div class="topic-group"><h2>' + escapeHtml(t) + '</h2>' + builder(list) + '</div>';
    }
    if (untagged.length) h += '<div class="topic-group"><h2>Other</h2>' + builder(untagged) + '</div>';
  } else {
    const list = byTopic.get(topic) || [];
    if (list.length) h = '<div class="topic-group"><h2>' + escapeHtml(topic) + '</h2>' + builder(list) + '</div>';
  }
  return h;
}

function wireTopicFilter(filterEl, onChange) {
  if (filterEl) filterEl.addEventListener('change', () => onChange(filterEl.value));
}

async function renderProgress(app) {
  app.innerHTML = '<p class="hint">Loading…</p>';
  const [exercises, sessions] = await Promise.all([api('exercises'), api('sessions')]);

  if (!exercises.length) {
    app.innerHTML = '<div class="empty"><h1>You haven\'t set up exercises yet</h1><p>Run <code>guitar-coach start</code> from your terminal to begin a timed session — the web UI shows your trend as you log sets.</p></div>';
    return;
  }

  // Progress points per exercise
  const pointsMap = {};
  for (const ex of exercises) {
    pointsMap[ex.id] = (await api('exercises/' + encodeURIComponent(ex.id) + '/progress')) || [];
  }

  const { topics, byTopic, untagged } = groupByTopic(exercises);

  const buildGrid = list => {
    let g = '<div class="exgrid">';
    for (const ex of list) g += exerciseCardHTML(ex, pointsMap[ex.id] || []);
    return g + '</div>';
  };

  let html = '<h1>Progress</h1>';
  html += topicFilterHTML(topics);

  const gridHost = document.createElement('div');
  gridHost.id = 'topic-groups';

  const renderGrid = topic => {
    gridHost.innerHTML = topicGroupsHTML(topics, byTopic, untagged, topic, buildGrid);
    wireNav(gridHost);
  };
  renderGrid('');

  if (sessions.length) html += renderStats(computeStats(sessions, exercises));

  app.innerHTML = html;
  app.appendChild(gridHost);

  const filterEl = document.getElementById('topic-filter');
  if (filterEl) filterEl.addEventListener('change', () => renderGrid(filterEl.value));
}

// ─── Exercises view ─────────────────────────────────────────────────────────

// Populates the sidebar submenu under Exercises with one link per topic.
function buildTopicSubnav(exercises) {
  const host = document.getElementById('subnav-exercises');
  if (!host) return;
  const { topics, byTopic, untagged } = groupByTopic(exercises);
  let html = '<div class="subnav-inner"><a href="/exercises" data-nav data-topic="all">All topics (' + exercises.length + ')</a>';
  for (const t of topics) {
    html += '<a href="/exercises?topic=' + encodeURIComponent(t) + '" data-nav data-topic="' + escapeHtml(t) + '">' +
      escapeHtml(t) + ' (' + (byTopic.get(t) || []).length + ')</a>';
  }
  if (untagged.length) html += '<a href="/exercises?topic=untagged" data-nav data-topic="untagged">Other (' + untagged.length + ')</a>';
  html += '</div>';
  host.innerHTML = html;
}

function exerciseListRow(ex, topicFiltered) {
  return '<a href="/exercises/' + escapeHtml(ex.id) + '" data-nav>' +
    '<span class="exname">' + escapeHtml(ex.name) + '</span>' +
    (ex.topic && !topicFiltered ? '<span class="pill">' + escapeHtml(ex.topic) + '</span>' : '') +
    (ex.description ? '<span class="desc">' + escapeHtml(ex.description) + '</span>' : '') +
    '</a>';
}

async function renderExercises(app, topic) {
  app.innerHTML = '<p class="hint">Loading…</p>';
  const exercises = await api('exercises');

  if (!exercises.length) {
    app.innerHTML = '<div class="empty"><h1>You haven\'t set up exercises yet</h1><p>Run <code>guitar-coach start</code> from your terminal to begin a timed session — exercises appear here once you have data.</p></div>';
    return;
  }

  buildTopicSubnav(exercises);
  const { topics, byTopic, untagged } = groupByTopic(exercises);
  const topicFiltered = topic === 'untagged' || (topic && byTopic.has(topic));

  const buildList = list => {
    let g = '<ul class="exlist">';
    for (const ex of list) g += '<li>' + exerciseListRow(ex, topicFiltered) + '</li>';
    return g + '</ul>';
  };

  const gridHost = document.createElement('div');
  let html = '<h1>Exercises</h1>';
  if (topicFiltered) {
    const label = topic === 'untagged' ? 'Other' : topic;
    html += '<p class="filter-note">Showing <span class="pill">' + escapeHtml(label) + '</span> ' +
      '<a href="/exercises" data-nav>clear filter \u00D7</a></p>';
    const list = topic === 'untagged' ? untagged : byTopic.get(topic);
    gridHost.innerHTML = '<div class="topic-group"><h2>' + escapeHtml(label) +
      '</h2>' + buildList(list) + '</div>';
  } else {
    gridHost.innerHTML = topicGroupsHTML(topics, byTopic, untagged, '', buildList);
  }
  wireNav(gridHost);

  app.innerHTML = html;
  app.appendChild(gridHost);
  setActiveNav();
}

// ─── Sessions view ──────────────────────────────────────────────────────────

const SESSION_PAGE_SIZE = 20;

async function renderSessions(app) {
  app.innerHTML = '<p class="hint">Loading…</p>';

  let page = 1, date = '';
  const tableHost = document.createElement('div');

  const load = async () => {
    const params = new URLSearchParams({ page: String(page), per_page: String(SESSION_PAGE_SIZE) });
    if (date) params.set('date', date);
    const res = await api('sessions?' + params.toString());

    if (!res.total) {
      tableHost.innerHTML = '<div class="empty"><h1>No sessions yet</h1><p>Run <code>guitar-coach start</code> in your terminal for your first timed session. Sessions land here when ended.</p></div>';
      return;
    }

    const pages = Math.max(1, Math.ceil(res.total / SESSION_PAGE_SIZE));
    let html = '<table><tr><th>When</th><th>Rounds</th><th>Exercises</th><th>Length</th></tr>';
    for (const s of res.sessions) {
      const maxRound = s.entries.reduce((m, e) => Math.max(m, e.round), 0);
      html += '<tr data-href="/sessions/' + escapeHtml(s.id) + '"><td>' +
        friendlyDay(s.started_at) + '</td><td>' +
        maxRound + '</td><td>' +
        s.entries.length + '</td><td>' +
        formatDuration(sessionDuration(s)) + '</td></tr>';
    }
    html += '</table>';

    if (pages > 1) {
      html += '<div class="pager"><span>';
      if (page > 1) html += '<a href="#" data-page="' + (page - 1) + '">\u2039 Prev</a>';
      else html += '<span class="dim">\u2039 Prev</span>';
      html += '</span><span>Page ' + page + ' of ' + pages + '</span><span>';
      if (page < pages) html += '<a href="#" data-page="' + (page + 1) + '">Next \u203A</a>';
      else html += '<span class="dim">Next \u203A</span>';
      html += '</span></div>';
    }

    tableHost.innerHTML = html;
    tableHost.querySelectorAll('a[data-page]').forEach(a => {
      a.addEventListener('click', e => {
        e.preventDefault();
        page = Number(a.dataset.page);
        load().then(() => window.scrollTo(0, 0));
      });
    });
    wireNav(tableHost);
  };

  let controls = '<h1>Sessions</h1>';
  controls += '<div class="chart-controls" style="margin-bottom:.9rem"><label>Date: <input type="date" id="session-date"></label></div>';

  app.innerHTML = controls;
  app.appendChild(tableHost);

  const dateEl = document.getElementById('session-date');
  if (dateEl) dateEl.addEventListener('change', () => {
    date = dateEl.value || '';
    page = 1;
    load();
  });

  await load();
}

// ─── Calendar view ──────────────────────────────────────────────────────────

async function renderCalendar(app) {
  app.innerHTML = '<p class="hint">Loading…</p>';
  const sessions = await api('sessions');

  if (!sessions.length) {
    app.innerHTML = '<div class="empty"><h1>No practice logged yet</h1><p>Finish a session with <code>guitar-coach start</code> and your practice calendar fills in here.</p></div>';
    return;
  }

  const hm = renderHeatmap(sessions);
  let html = '<h1>Practice calendar</h1>';
  if (hm) {
    html += hm;
    html += '<div class="hm-legend">less <i></i><i class="l1"></i><i class="l2"></i><i class="l3"></i><i class="l4"></i> more</div>';
  }

  app.innerHTML = html;
}

function wireNav(root) {
  root.querySelectorAll('a[data-nav]').forEach(a => {
    a.addEventListener('click', e => { e.preventDefault(); navigate(a.getAttribute('href')); });
  });
  root.querySelectorAll('tr[data-href]').forEach(tr => {
    tr.addEventListener('click', e => {
      if (e.target.closest('a')) return;
      navigate(tr.dataset.href);
    });
  });
}

// ─── uPlot chart ────────────────────────────────────────────────────────────

function renderUPlot(points, container, height) {
  container.innerHTML = '';
  if (!points || !points.length) {
    container.innerHTML = '<p class="hint">No data yet for this exercise.</p>';
    return;
  }
  points = points.slice().sort((a, b) => parseDate(a.date) - parseDate(b.date));

  let best = 0;
  const pr = points.map(p => { const isNew = p.end_bpm > best; if (p.end_bpm > best) best = p.end_bpm; return isNew; });
  const data = [[], [], []];
  for (const p of points) {
    data[0].push(parseDate(p.date));
    data[1].push(p.start_bpm > 0 ? p.start_bpm : null);
    data[2].push(p.end_bpm > 0 ? p.end_bpm : null);
  }

  const cap = document.createElement('p');
  cap.className = 'hint';
  cap.textContent = best > 0
    ? 'best ' + best + ' bpm \u2014 gold points are personal records'
    : 'hover the chart to inspect values';
  container.appendChild(cap);

  const target = document.createElement('div');
  container.appendChild(target);

  const palette = { start: '#eba07e', end: '#e8996a', gold: '#e2bd7c' };

  container.style.position = 'relative';
  const tip = document.createElement('div');
  tip.className = 'chart-note';
  container.appendChild(tip);

  new uPlot({
    width: target.clientWidth || 760,
    height: height || 320,
    padding: [14, 14, 8, 8],
    scales: { x: { time: true }, y: { auto: true, range: (u, min, max) => {
      const pad = Math.max(4, Math.round((max - min) * 0.18));
      return pad ? [min - pad, max + pad] : [min, max];
    } } },
    legend: { show: true },
    cursor: { x: true, y: false, stroke: 'rgba(255,255,255,.35)', width: 1, dash: [4, 4] },
    axes: [
      { stroke: '#8b857b', font: '11px system-ui', size: 34,
        values: (u, ticks) => ticks.map(t => t == null ? '' : fmtDate(t)),
        grid: { stroke: 'rgba(217,123,86,.09)', width: 1, dash: [3, 5] },
        ticks: { stroke: '#403c44' } },
      { stroke: '#8b857b', font: '11px system-ui', size: 26, label: 'BPM',
        grid: { stroke: 'rgba(217,123,86,.09)', width: 1, dash: [3, 5] },
        ticks: { stroke: '#403c44' } }
    ],
    hooks: {
      setCursor: [u => {
        const idx = u.cursor.idx;
        if (idx == null || idx < 0 || !points[idx]) { tip.style.display = 'none'; return; }
        const p = points[idx];
        tip.innerHTML = '<b>' + fmtDate(data[0][idx]) + '</b> \u00B7 ' +
          (p.start_bpm > 0 ? p.start_bpm + ' \u2192 ' : '') +
          (p.end_bpm > 0 ? p.end_bpm + ' bpm' : '\u2014');
        tip.style.display = 'block';
        const overRect = u.over.getBoundingClientRect();
        const baseRect = container.getBoundingClientRect();
        const gap = 12;
        let l = overRect.left - baseRect.left + u.cursor.left + gap;
        let t = overRect.top - baseRect.top + u.cursor.top + gap;
        tip.style.left = l + 'px';
        tip.style.top = t + 'px';
        const maxL = container.clientWidth - tip.offsetWidth - 6;
        if (l > maxL) tip.style.left = Math.max(0, overRect.left - baseRect.left + u.cursor.left - tip.offsetWidth - gap) + 'px';
        const maxT = container.clientHeight - tip.offsetHeight - 6;
        if (t > maxT) tip.style.top = Math.max(0, overRect.top - baseRect.top + u.cursor.top - tip.offsetHeight - gap) + 'px';
      }]
    },
    series: [
      { label: 'date', value: (u, ts) => ts == null ? '-' : fmtDate(ts) },
      { label: 'start bpm', stroke: palette.start, width: 1.5, spanGaps: true,
        fill: 'rgba(217,123,86,.06)',
        points: { show: true, size: 4, stroke: palette.start, fill: '#1a1920' },
        value: (u, raw) => raw == null ? '-' : raw + ' bpm' },
      { label: 'end bpm', stroke: palette.end, width: 2.5, spanGaps: true,
        fill: 'rgba(232,153,106,.10)',
        value: (u, raw) => raw == null ? '-' : raw + ' bpm',
        points: {
          show: true,
          size: (u, i) => pr[i] ? 7 : 4,
          stroke: (u, i) => pr[i] ? palette.gold : palette.end,
          fill: (u, i) => pr[i] ? 'rgba(255,212,94,.25)' : '#1a1920'
        } }
    ]
  }, data, target);
}

// ─── Exercise history ───────────────────────────────────────────────────────

async function renderExerciseHistory(app, id) {
  app.innerHTML = '<p class="hint">Loading…</p>';
  const [exercises, progress] = await Promise.all([
    api('exercises'),
    api('exercises/' + encodeURIComponent(id) + '/progress').catch(() => [])
  ]);
  const ex = exercises.find(e => e.id === id);

  let html = '<a href="/" data-nav style="font-size:.9rem">\u2190 dashboard</a>';
  if (!ex) {
    html += '<div class="empty"><h1>Exercise not found</h1></div>';
    app.innerHTML = html;
    return;
  }
  html += '<h1>' + escapeHtml(ex.name) + '</h1>';
  if (ex.topic) html += '<p><span class="pill">' + escapeHtml(ex.topic) + '</span></p>';
  if (ex.description) html += '<p class="desc">' + escapeHtml(ex.description) + '</p>';

  if (!progress.length) {
    html += '<p>No history yet for this exercise.</p>';
  } else {
    html += '<div class="chart" id="exhist-chart"></div>';
    html += '<p class="hint">' + progress.length + ' recorded sets</p>';
    html += '<table><tr><th>When</th><th>Session</th><th>Round</th><th>Start</th><th>End</th><th>Notes</th></tr>';
    for (let i = progress.length - 1; i >= 0; i--) {
      const p = progress[i];
      const start = p.start_bpm > 0 ? p.start_bpm : '—';
      const end = p.end_bpm > 0 ? p.end_bpm : '—';
      const notes = p.notes ? escapeHtml(p.notes) : '';
      html += '<tr><td>' + friendlyDay(p.date) + '</td><td><a href="/sessions/' + escapeHtml(p.session_id) + '" data-nav>' + p.session_id.slice(0, 12) + '</a></td><td>r' + p.round + '</td><td>' + start + '</td><td>' + end + '</td><td>' + notes + '</td></tr>';
    }
    html += '</table>';
  }

  app.innerHTML = html;
  const chartC = document.getElementById('exhist-chart');
  if (chartC) renderUPlot(progress, chartC, 260);
  wireNav(app);
}

async function renderSessionDetail(app, id) {
  app.innerHTML = '<p class="hint">Loading…</p>';
  const [session, exercises] = await Promise.all([
    api('sessions/' + encodeURIComponent(id)),
    api('exercises')
  ]);

  const nameByID = {};
  for (const ex of exercises) nameByID[ex.id] = ex.name;

  let html = '<a href="/" data-nav style="font-size:.9rem">\u2190 dashboard</a>';
  html += '<h1>session ' + escapeHtml(id) + '</h1>';
  html += '<p>started ' + friendlyDay(session.started_at);
  if (session.ended_at) html += ' \u00B7 ended ' + friendlyDay(session.ended_at);
  const dur = sessionDuration(session);
  if (dur > 0) html += ' \u00B7 length ' + formatDuration(dur);
  html += '</p>';

  html += '<p>plan: ' + (session.config.exercises_per_round || 5) + ' exercises, ' +
    formatDuration(session.config.duration_sec || 0) + ' each, ' +
    formatDuration(session.config.rest_sec || 0) + ' rest, ' +
    formatDuration(session.config.break_sec || 0) + ' break</p>';

  if (session.order && session.order.length) {
    const names = session.order.map(id => nameByID[id] || id).filter(Boolean);
    if (names.length) html += '<p>order: ' + escapeHtml(names.join(' \u2192 ')) + '</p>';
  }

  let round = 0;
  for (let i = 0; i < (session.entries || []).length; i++) {
    const e = session.entries[i];
    if (e.round !== round) { round = e.round; html += '<h2>round ' + round + '</h2>'; }
    const name = e.name || nameByID[e.exercise_id] || e.exercise_id;
    html += '<div class="entry"><span>' + e.sequence + '.</span> <span class="exname">' + escapeHtml(name) + '</span>';
    html += '<input type="number" data-idx="' + i + '" data-field="start_bpm" value="' + (e.start_bpm || 0) + '" title="start bpm">';
    html += ' \u2192 ';
    html += '<input type="number" data-idx="' + i + '" data-field="end_bpm" value="' + (e.end_bpm || 0) + '" title="end bpm">';
    html += ' bpm';
    html += '<input type="text" data-idx="' + i + '" data-field="notes" value="' + escapeHtml(e.notes || '') + '" placeholder="notes">';
    html += '<span class="saved">saved</span></div>';
  }

  // Per-exercise history
  const seen = new Set();
  const uniq = [];
  for (const id of (session.order || [])) {
    if (!id || seen.has(id)) continue;
    seen.add(id);
    const ex = exercises.find(e => e.id === id);
    if (!ex) continue;
    uniq.push(ex);
  }
  if (uniq.length) {
    html += '<h2>Exercise history</h2>';
    html += '<p class="hint">Progress of each exercise across all its sessions, not just this one.</p>';
    const chartIDs = [];
    for (const ex of uniq) {
      html += '<div class="exblock"><h3>' + escapeHtml(ex.name);
      if (ex.topic) html += ' <span class="pill">' + escapeHtml(ex.topic) + '</span>';
      html += '</h3>';
      if (ex.description) html += '<p class="desc">' + escapeHtml(ex.description) + '</p>';
      const cid = 'exhist-chart-' + ex.id;
      html += '<div class="chart" id="' + cid + '"></div>';
      chartIDs.push({ id: ex.id, cid: cid });
      html += '</div>';
    }
    app.innerHTML = html;
    for (const { id, cid } of chartIDs) {
      const pts = await api('exercises/' + encodeURIComponent(id) + '/progress');
      const c = document.getElementById(cid);
      if (c) renderUPlot(pts, c, 240);
    }
    wireNav(app);
    wireEntryEditing(session.id);
    return;
  }

  app.innerHTML = html;
  wireNav(app);
  wireEntryEditing(session.id);
}

// ─── Entry editing ──────────────────────────────────────────────────────────

function wireEntryEditing(sessionId) {
  document.querySelectorAll('.entry [data-field]').forEach(field => {
    field.addEventListener('change', () => {
      const idx = field.dataset.idx;
      const body = {};
      body[field.dataset.field] = field.type === 'number'
        ? (field.value === '' ? 0 : Number(field.value))
        : field.value;
      fetch('/api/sessions/' + encodeURIComponent(sessionId) + '/entries/' + encodeURIComponent(idx), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      }).then(res => {
        if (res.ok) {
          const entry = field.closest('.entry');
          entry.classList.add('saving');
          setTimeout(() => entry.classList.remove('saving'), 1400);
        } else {
          alert('save failed (HTTP ' + res.status + ')');
        }
      });
    });
  });
}

// ─── Init ───────────────────────────────────────────────────────────────────

window.addEventListener('popstate', route);
document.addEventListener('click', e => {
  const parent = e.target.closest('.nav-parent');
  if (parent) {
    e.preventDefault();
    const group = document.getElementById('navg-exercises');
    if (location.pathname === '/exercises') {
      group.classList.toggle('open');
    } else {
      navigate('/exercises');
    }
    return;
  }
  const a = e.target.closest('a[data-nav]');
  if (a) { e.preventDefault(); navigate(a.getAttribute('href')); }
});
route();
