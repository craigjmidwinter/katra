// Dev-server only: reloads the tab when the watcher reports a change. Never
// copied into a static build.
(function () {
  if (location.search.indexOf("noreload") !== -1) return;
  var es = new EventSource("/__livereload");
  es.onmessage = function (e) { if (e.data === "reload") location.reload(); };
})();
