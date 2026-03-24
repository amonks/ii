// Minimal stacked bar chart renderer for tagtime graphs.
(function() {
  const canvas = document.getElementById('chart');
  if (!canvas) return;

  const params = new URLSearchParams(window.location.search);
  const base = document.querySelector('meta[name="base-path"]')?.content || '';
  const window_ = params.get('window') || 'day';
  const start = params.get('start') || '';
  const end = params.get('end') || '';

  let url = base + '/graphs/data?window=' + window_;
  if (start) url += '&start=' + start;
  if (end) url += '&end=' + end;

  fetch(url)
    .then(r => r.json())
    .then(data => render(canvas, data))
    .catch(err => console.error('chart error:', err));

  function render(canvas, data) {
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.scale(dpr, dpr);

    const w = rect.width;
    const h = rect.height;
    const padding = { top: 20, right: 20, bottom: 40, left: 60 };
    const plotW = w - padding.left - padding.right;
    const plotH = h - padding.top - padding.bottom;

    if (!data.buckets || data.buckets.length === 0) {
      ctx.fillStyle = '#8899aa';
      ctx.font = '14px sans-serif';
      ctx.fillText('No data', w / 2 - 25, h / 2);
      return;
    }

    const colors = data.tag_colors || {};

    // Find max total per bucket for scale.
    let maxTotal = 0;
    for (const b of data.buckets) {
      let total = 0;
      for (const tag of (data.all_tags || [])) {
        total += (b.tags[tag] || 0);
      }
      if (total > maxTotal) maxTotal = total;
    }
    if (maxTotal === 0) maxTotal = 1;

    const barW = plotW / data.buckets.length;
    const gap = Math.max(1, barW * 0.1);
    const tags = data.all_tags || [];

    // Draw bars.
    for (let i = 0; i < data.buckets.length; i++) {
      const b = data.buckets[i];
      let y = padding.top + plotH;
      const x = padding.left + i * barW + gap / 2;

      for (const tag of tags) {
        const val = b.tags[tag] || 0;
        const barH = (val / maxTotal) * plotH;
        y -= barH;
        ctx.fillStyle = colors[tag] || '#888';
        ctx.fillRect(x, y, barW - gap, barH);
      }
    }

    // Y-axis: hours.
    ctx.fillStyle = '#8899aa';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'right';
    for (let i = 0; i <= 4; i++) {
      const val = (maxTotal / 4) * i;
      const y = padding.top + plotH - (val / maxTotal) * plotH;
      const hours = (val / 3600).toFixed(1);
      ctx.fillText(hours + 'h', padding.left - 5, y + 4);
    }

    // X-axis: bucket labels.
    ctx.textAlign = 'center';
    const labelStep = Math.max(1, Math.floor(data.buckets.length / 8));
    for (let i = 0; i < data.buckets.length; i += labelStep) {
      const b = data.buckets[i];
      const x = padding.left + i * barW + barW / 2;
      const d = new Date(b.start);
      let label;
      if (data.window === 'hour') {
        label = d.getHours() + ':00';
      } else {
        label = (d.getMonth() + 1) + '/' + d.getDate();
      }
      ctx.fillText(label, x, h - padding.bottom + 15);
    }

    // Legend.
    let legendX = padding.left;
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    for (const tag of tags) {
      ctx.fillStyle = colors[tag] || '#888';
      ctx.fillRect(legendX, h - 12, 10, 10);
      ctx.fillStyle = '#8899aa';
      ctx.fillText(tag, legendX + 14, h - 3);
      legendX += ctx.measureText(tag).width + 28;
    }
  }
})();
