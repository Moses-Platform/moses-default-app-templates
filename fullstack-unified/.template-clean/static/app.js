(function () {
  // ── MOSES ROUTING ──────────────────────────────────────────────────────
  // All API calls MUST use the pattern:  fetch(base + '/api/v1/...')
  //
  // index.html hardcodes <base href="./">, so the browser resolves
  // document.querySelector('base').href to the FULL absolute URL of the
  // current page's directory — origin AND tenant/chart subpath included,
  // e.g. "https://host/apps/tenant/chart/". The relative "./" is what makes
  // it deployment-agnostic.
  //
  // The leading '/' that follows `base` is safe ONLY because `base` is an
  // absolute URL after resolution — the result is an absolute URL, NOT a
  // root-relative path.
  //
  //   CORRECT:  fetch(base + '/api/v1/yourEndpoint')
  //   WRONG:    fetch('/api/v1/yourEndpoint')   ← bypasses subpath, will 404
  //
  // The template demo was removed by clean_out_template.sh. Example wiring:
  //
  //   fetch(base + '/api/v1/yourEndpoint')
  //     .then(function (r) {
  //       if (!r.ok) throw new Error('HTTP ' + r.status);
  //       return r.json();
  //     })
  //     .then(render)
  //     .catch(function (e) { showError('Failed: ' + e.message); });
  //
  // CHAT-w6gt: the backend never emits tenant UUIDs in response bodies —
  // don't render or expect them here.
  // ────────────────────────────────────────────────────────────────────────
  var base = document.querySelector('base').href.replace(/\/$/, '');
  void base; // referenced by your fetch calls above
})();
