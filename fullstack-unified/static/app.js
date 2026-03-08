(function () {
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
