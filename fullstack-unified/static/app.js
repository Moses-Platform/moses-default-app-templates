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
  //   CORRECT:  fetch(base + '/api/v1/things')
  //   WRONG:    fetch('/api/v1/things')   ← bypasses subpath, will 404
  //
  // Example wiring — the browser half of the `things` vertical slice. It stays
  // a comment because this file is SERVED as-is (nothing compiles or strips
  // it). The backend half is REAL, CI-compiled code in example_test.go at the
  // module root (routes + handlers); the "/things" spec you add to
  // api/openapi.json is worked out in the comment above the
  // //go:embed api/openapi.json directive in main.go. Rename things/Thing
  // to your real resource:
  //
  //   // The two sinks the fetch below uses. This file is SERVED as-is —
  //   // an example calling helpers that do not exist is a runtime
  //   // ReferenceError in the browser, so uncomment these with it.
  //   function render(things) {
  //     var list = document.getElementById('things');
  //     if (!list) return;
  //     list.textContent = '';
  //     (things || []).forEach(function (t) {
  //       var li = document.createElement('li');
  //       li.textContent = t.name;
  //       list.appendChild(li);
  //     });
  //   }
  //
  //   function showError(message) {
  //     var box = document.getElementById('error');
  //     if (box) box.textContent = message;
  //   }
  //
  //   // The backend answers {"things": [...]} — an OBJECT, not a bare
  //   // array (same shape as api/openapi.json and demo_routes.go).
  //   fetch(base + '/api/v1/things')
  //     .then(function (r) {
  //       if (!r.ok) throw new Error('HTTP ' + r.status);
  //       return r.json();
  //     })
  //     .then(function (data) { render(data.things); })
  //     .catch(function (e) { showError('Failed: ' + e.message); });
  //
  //   // Write. No CSRF token needed: the Go guard is Sec-Fetch-Site-based
  //   // (csrf.go), so a same-origin fetch passes with no extra header.
  //   fetch(base + '/api/v1/things', {
  //     method: 'POST',
  //     headers: { 'Content-Type': 'application/json' },
  //     body: JSON.stringify({ name: 'New thing' })
  //   });
  //
  // CHAT-w6gt: the backend never emits tenant UUIDs in response bodies —
  // don't render or expect them here.
  // ────────────────────────────────────────────────────────────────────────
  var base = document.querySelector('base').href.replace(/\/$/, '');
  void base; // referenced by your fetch calls above
})();
