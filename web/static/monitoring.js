'use strict';
const monitorRoot = document.getElementById('monitor-charts');
if (monitorRoot) {
  const fa = document.documentElement.lang === 'fa';
  const label = (en, faText) => fa ? faText : en;
  const status = document.getElementById('monitor-status');
  const auto = document.getElementById('monitor-auto');
  let busy = false;
  const metrics = [
    ['cpu', 'CPU', '#f59e0b', '%', 100],
    ['memory', label('Memory used','حافظه مصرف‌شده'), '#38bdf8', '%', 100],
    ['disk', label('Disk used','فضای دیسک مصرف‌شده'), '#a78bfa', '%', 100],
    ['rx', label('Physical interfaces RX','دریافت رابط‌های فیزیکی'), '#34d399', 'Mbps'],
    ['tx', label('Physical interfaces TX','ارسال رابط‌های فیزیکی'), '#fb7185', 'Mbps'],
    ['ppp', label('PPP sessions','نشست‌های PPP'), '#fbbf24', ''],
    ['openvpn', label('OpenVPN sessions','نشست‌های OpenVPN'), '#fb923c', ''],
    ['sstp', label('SSTP sessions','نشست‌های SSTP'), '#c084fc', ''],
    ['l2tp', label('L2TP/IPsec sessions','نشست‌های L2TP/IPsec'), '#22d3ee', ''],
    ['wireguard', label('Recent WireGuard handshakes','هندشیک‌های اخیر WireGuard'), '#2dd4bf', ''],
    ['ipsec', label('Active IPsec peers','peerهای فعال IPsec'), '#818cf8', ''],
    ['online', label('Router reachability','دسترسی به روتر'), '#4ade80', '', 1]
  ];
  const make = (tag, text, className) => { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (className) el.className = className; return el; };
  const jobStates = document.getElementById('monitor-job-states');
  if (jobStates) {
    jobStates.append(make('h3', label('Provisioning job states','وضعیت عملیات پیکربندی')));
    const bars = make('div', undefined, 'job-state-bars');
    for (const [state, count] of Object.entries(JSON.parse(jobStates.dataset.counts || '{}'))) bars.append(make('span', state + ' · ' + count));
    jobStates.append(bars);
    const counts = Object.values(JSON.parse(jobStates.dataset.counts || '{}')).map(Number);
    if (counts.length) {
      const canvas = make('canvas'); canvas.width = 1000; canvas.height = 220; canvas.className = 'job-state-canvas'; canvas.setAttribute('role','img'); canvas.setAttribute('aria-label',label('Job counts by state','تعداد عملیات به تفکیک وضعیت'));
      const ctx = canvas.getContext('2d'), max = Math.max(1,...counts), colors = ['#f59e0b','#34d399','#fb7185','#38bdf8','#a78bfa'];
      counts.forEach((count,index) => { const width = 960/counts.length; ctx.fillStyle=colors[index%colors.length]; ctx.fillRect(20+index*width,200-count/max*180,Math.max(2,width-12),count/max*180); });
      jobStates.append(canvas);
    }
  }
  function valuesFor(history, key) {
    return history.map((point, index) => {
      if (key === 'online') return point.online ? 1 : 0;
      if (!point.online) return null;
      if (key === 'rx' || key === 'tx') {
        const prev = history[index - 1], field = key + '_bytes';
        if (!prev?.online || point[field] == null || prev[field] == null) return null;
        const seconds = (Date.parse(point.at) - Date.parse(prev.at)) / 1000;
        const delta = point[field] - prev[field];
        return seconds > 0 && seconds <= 300 && delta >= 0 ? delta * 8 / seconds / 1e6 : null;
      }
      return point[key] ?? null;
    });
  }
  function draw(canvas, points, color, ceiling, times) {
    const width = 500, height = 130;
    canvas.width = width * 2; canvas.height = height * 2;
    const ctx = canvas.getContext('2d'); ctx.scale(2, 2);
    const finite = points.filter(v => Number.isFinite(v));
    const max = ceiling || Math.max(1, ...finite) * 1.15;
    ctx.strokeStyle = '#88888844'; ctx.lineWidth = 1;
    for (let line = 0; line < 4; line++) { const y = 10 + line * 33; ctx.beginPath(); ctx.moveTo(35,y); ctx.lineTo(490,y); ctx.stroke(); }
    ctx.fillStyle = '#aaa'; ctx.font = '10px Tahoma'; ctx.fillText(String(Number(max.toFixed(1))), 2, 13); ctx.fillText('0', 15, 113);
    const first = times[0] || 0, span = (times[times.length-1] || first) - first;
    const xFor = i => span > 0 ? 38 + (times[i]-first) / span * 447 : 264;
    ctx.strokeStyle = color; ctx.lineWidth = 2.5; ctx.beginPath(); let active = false;
    points.forEach((value,i) => {
      if (!Number.isFinite(value)) { active = false; return; }
      const x = xFor(i), y = 110 - Math.min(value / max, 1) * 100;
      if (i > 0 && times[i]-times[i-1] > 300000) active = false;
      if (active) ctx.lineTo(x,y); else ctx.moveTo(x,y); active = true;
    }); ctx.stroke();
    ctx.fillStyle = color;
    points.forEach((value,i) => { if (Number.isFinite(value)) { ctx.beginPath(); ctx.arc(xFor(i),110-Math.min(value/max,1)*100,3,0,Math.PI*2); ctx.fill(); } });
  }
  function render(routers) {
    monitorRoot.replaceChildren();
    for (const router of routers) {
      const section = make('section', undefined, 'monitor-router');
      section.append(make('h2', router.name + ' · ' + router.status));
      const history = router.history || [], times = history.map(p => Date.parse(p.at));
      const range = history.length ? new Date(history[0].at).toLocaleTimeString() + ' — ' + new Date(history[history.length-1].at).toLocaleTimeString() : label('No samples','هنوز نمونه‌ای ثبت نشده');
      section.append(make('p', range + ' · ' + history.length + label(' samples',' نمونه')));
      const grid = make('div', undefined, 'monitor-grid');
      for (const [key, title, color, unit, ceiling] of metrics) {
        const points = valuesFor(history,key), latest = points[points.length-1];
        const card = make('article', undefined, 'glass-card monitor-chart');
        card.append(make('h3', title));
        card.append(make('strong', Number.isFinite(latest) ? Number(latest.toFixed(2)).toLocaleString() + ' ' + unit : label('Unavailable','داده موجود نیست')));
        const canvas = make('canvas'); canvas.setAttribute('role','img'); canvas.setAttribute('aria-label',title + ' · ' + range);
        card.append(canvas); draw(canvas,points,color,ceiling,times);
        if (!points.some(v => Number.isFinite(v))) card.append(make('small', label('No observed data; counters may be unsupported or require another sample.','داده دریافت نشده؛ شمارنده ممکن است پشتیبانی نشود یا به نمونه بعدی نیاز داشته باشد.')));
        grid.append(card);
      }
      section.append(grid);
      const latest = history[history.length-1];
      if (latest?.warnings?.length) section.append(make('p',label('Unavailable menus: ','منوهای بدون داده: ') + latest.warnings.join(', '),'notice'));
      monitorRoot.append(section);
    }
  }
  async function update(sample) {
    if (busy || document.hidden) return;
    busy = true;
    try {
      let response = await fetch('/api/v1/monitoring', {headers:{Accept:'application/json'}});
      if (!response.ok) throw new Error(String(response.status));
      let data = await response.json(); let failures = 0;
      if (sample) {
        for (const router of data.routers) {
          if (!router.can_sync) continue;
          const sync = await fetch('/api/v1/routers/' + encodeURIComponent(router.id) + '/sync', {method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':document.querySelector('meta[name="csrf-token"]').content},body:'{}'});
          if (!sync.ok) failures++;
        }
        response = await fetch('/api/v1/monitoring', {headers:{Accept:'application/json'}}); if (!response.ok) throw new Error(String(response.status)); data = await response.json();
      }
      render(data.routers);
      status.textContent = label('Updated: ','به‌روزرسانی: ') + new Date().toLocaleTimeString() + (failures ? label(' · Some routers could not be sampled',' · دریافت اطلاعات برخی روترها ناموفق بود') : '');
    } catch (_) { status.textContent = label('Monitoring refresh failed; previous graphs are preserved.','به‌روزرسانی مانیتورینگ ناموفق بود؛ گراف‌های قبلی حفظ شدند.'); }
    finally { busy = false; }
  }
  document.getElementById('monitor-sample')?.addEventListener('click', () => update(true));
  update(false);
  setInterval(() => update(auto?.checked === true), 30000);
}
