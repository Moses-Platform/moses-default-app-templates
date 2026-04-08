(function () {
  // ── MOSES ROUTING ──────────────────────────────────────────────────────
  // This file uses the pattern:  fetch(base + '/api/v1/...')
  //
  // `base` is resolved from the <base href="..."> tag injected by the
  // Moses static-serve layer. It already contains the full origin AND the
  // tenant/chart subpath, e.g. "https://host/apps/tenant/chart".
  //
  // The leading '/' that follows `base` is safe ONLY because `base`
  // includes the full origin+subpath — the browser treats the result as
  // an absolute URL, NOT a root-relative path.
  //
  // If you add new fetch calls you MUST use the same pattern:
  //   CORRECT:  fetch(base + '/my/endpoint')
  //   WRONG:    fetch('/api/v1/something')   ← bypasses subpath, will 404
  // ────────────────────────────────────────────────────────────────────────
  var base = document.querySelector('base').href.replace(/\/$/, '');

  function render(data) {
    var el = document.getElementById('status');
    var rows = [
      ['App', data.app],
      ['Version', data.version],
      ['Uptime', data.uptime],
      ['Port', (data.env && data.env.port) || '—'],
      ['Base URL', (data.env && data.env.base_url) || '/'],
    ];
    if (data.moses && data.moses.tenant_id) {
      rows.push(['Tenant', data.moses.tenant_id]);
    }
    el.textContent = '';
    rows.forEach(function (r) {
      var row = document.createElement('div');
      row.className = 'row';
      var label = document.createElement('span');
      label.className = 'label';
      label.textContent = r[0];
      var value = document.createElement('span');
      value.className = 'value mono';
      value.textContent = r[1];
      row.appendChild(label);
      row.appendChild(value);
      el.appendChild(row);
    });
    el.hidden = false;
    document.getElementById('loading').hidden = true;
  }

  function showError(msg) {
    var el = document.getElementById('error');
    el.textContent = msg;
    el.hidden = false;
    document.getElementById('loading').hidden = true;
  }

  // base already contains origin+subpath from <base href>, so this
  // resolves to e.g. "https://host/apps/tenant/chart/api/v1/status"
  fetch(base + '/api/v1/status')
    .then(function (r) {
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.json();
    })
    .then(render)
    .catch(function (e) {
      showError('Failed to connect: ' + e.message);
    });
})();
