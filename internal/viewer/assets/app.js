// Dev-log viewer. Zero dependencies. Reads data.json (entries pre-rendered to
// HTML in Go) and lays them out: drafts → "In Progress", featured → "Deep
// Dives", the rest → "The Log". Tag filter, image lightbox, before/after slider.
(function () {
  "use strict";

  var state = { data: null, tag: null };

  function el(tag, cls, html) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (html != null) n.innerHTML = html;
    return n;
  }

  function esc(s) {
    return (s || "").replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  function load() {
    fetch("data.json", { cache: "no-store" })
      .then(function (r) { return r.json(); })
      .then(function (d) { state.data = d; render(); })
      .catch(function (e) { console.error("devlog: failed to load data.json", e); });
  }

  function render() {
    var d = state.data;
    if (!d) return;
    var accent = d.site && d.site.accent;
    if (accent) document.documentElement.style.setProperty("--accent", accent);
    document.title = (d.site && d.site.title) || "Dev Log";
    setText("site-title", (d.site && d.site.title) || "Dev Log");
    setText("site-desc", (d.site && d.site.description) || "");

    var entries = (d.entries || []).filter(function (e) {
      return !state.tag || (e.tags || []).indexOf(state.tag) >= 0;
    });

    var drafts = entries.filter(function (e) { return e.draft; });
    var featured = entries.filter(function (e) { return !e.draft && e.featured; });
    var log = entries.filter(function (e) { return !e.draft && !e.featured; });

    fill("drafts", drafts);
    fill("featured", featured);
    fill("log", log);

    toggle("drafts-section", drafts.length > 0);
    toggle("featured-section", featured.length > 0);
    toggle("log-section", log.length > 0);
    document.getElementById("empty").hidden = entries.length > 0;

    setText("count-logged", (d.entries || []).filter(function (e) { return !e.draft; }).length + " logged");
    setText("count-drafts", (d.entries || []).filter(function (e) { return e.draft; }).length + " in progress");

    renderTagbar();
    wireInteractions();
  }

  function fill(id, list) {
    var box = document.getElementById(id);
    box.innerHTML = "";
    list.forEach(function (e) { box.appendChild(card(e)); });
  }

  function card(e) {
    var c = el("article", "dl-card" + (e.draft ? " is-draft" : ""));
    c.id = "e-" + e.slug;

    if (e.cover) {
      var cov = el("img", "dl-card-cover");
      cov.src = e.cover; cov.alt = e.title; cov.setAttribute("data-dl-lightbox", "");
      c.appendChild(cov);
    }

    var head = el("div", "dl-card-head");
    var row = el("div", "dl-card-titlerow");
    row.appendChild(el("h3", "dl-card-title", esc(e.title)));
    var when = (e.date || "") + (e.time ? " " + e.time.slice(0, 5) : "");
    row.appendChild(el("span", "dl-card-date", esc(when)));
    head.appendChild(row);
    if (e.tags && e.tags.length) {
      var tags = el("div", "dl-card-tags");
      e.tags.forEach(function (t) { tags.appendChild(el("span", "dl-chip", esc(t))); });
      head.appendChild(tags);
    }
    c.appendChild(head);

    c.appendChild(el("div", "dl-card-body", e.html || ""));
    c.appendChild(foot(e));
    return c;
  }

  function foot(e) {
    var f = el("div", "dl-card-foot");
    if (e.draft) {
      f.appendChild(el("span", "dl-draft-badge", "● in progress — awaiting commit stamp"));
      return f;
    }
    if (e.hashes && e.hashes.length) {
      var label = e.hashes.length > 1 ? "commits" : "commit";
      f.appendChild(el("span", "dl-hash", label + " <b>" + e.hashes.map(esc).join("</b> <b>") + "</b>"));
    }
    if (e.stat) {
      var s = el("span", "dl-diffstat");
      s.innerHTML =
        '<span class="f">' + e.stat.f + " files</span>" +
        '<span class="a">+' + e.stat.a + "</span>" +
        '<span class="d">−' + e.stat.d + "</span>";
      f.appendChild(s);
    }
    return f;
  }

  function renderTagbar() {
    var bar = document.getElementById("tagbar");
    bar.innerHTML = "";
    var counts = {};
    (state.data.entries || []).forEach(function (e) {
      (e.tags || []).forEach(function (t) { counts[t] = (counts[t] || 0) + 1; });
    });
    var tags = Object.keys(counts).sort(function (a, b) { return counts[b] - counts[a]; });
    if (!tags.length) return;

    var all = el("button", "dl-tag-btn" + (state.tag ? "" : " active"), "all");
    all.onclick = function () { state.tag = null; render(); };
    bar.appendChild(all);
    tags.forEach(function (t) {
      var b = el("button", "dl-tag-btn" + (state.tag === t ? " active" : ""), esc(t));
      b.onclick = function () { state.tag = state.tag === t ? null : t; render(); };
      bar.appendChild(b);
    });
  }

  function wireInteractions() {
    document.querySelectorAll("[data-dl-lightbox], .dl-card-body img, .dl-card-cover").forEach(function (img) {
      if (img.closest(".dl-compare")) return;
      img.onclick = function () { openLightbox(img.src); };
    });
    document.querySelectorAll("[data-dl-compare]").forEach(wireCompare);
  }

  function wireCompare(fig) {
    var range = fig.querySelector(".dl-compare-range");
    var before = fig.querySelector(".dl-compare-before");
    var handle = fig.querySelector(".dl-compare-handle");
    function set(v) {
      before.style.width = v + "%";
      handle.style.left = v + "%";
    }
    range.addEventListener("input", function () { set(range.value); });
    set(range.value);
  }

  function openLightbox(src) {
    var lb = document.getElementById("lightbox");
    document.getElementById("lightbox-img").src = src;
    lb.hidden = false;
  }
  function closeLightbox() {
    document.getElementById("lightbox").hidden = true;
    document.getElementById("lightbox-img").src = "";
  }

  function setText(id, t) { var n = document.getElementById(id); if (n) n.textContent = t; }
  function toggle(id, on) { var n = document.getElementById(id); if (n) n.hidden = !on; }

  document.addEventListener("DOMContentLoaded", function () {
    document.getElementById("lightbox").onclick = closeLightbox;
    document.getElementById("lightbox-close").onclick = closeLightbox;
    document.addEventListener("keydown", function (e) { if (e.key === "Escape") closeLightbox(); });
    load();
  });
})();
