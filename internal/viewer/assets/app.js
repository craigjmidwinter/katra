// Dev-log viewer. Zero dependencies. Reads data.json (nodes pre-rendered to HTML
// in Go) and lays them out. Entries flow through the classic zones — drafts →
// "In Progress", featured → "Deep Dives", the rest → "The Log". The Katra node
// model adds forward-looking views: a Board (tasks by status), a Roadmap (epics
// & loose tasks by horizon), Decisions and Reference (articles), plus a
// per-node backlinks panel. Every node card carries id="<slug>" so in-body
// [[wikilinks]] (rendered to <a href="#slug">) scroll to their target.
(function () {
  "use strict";

  var state = { data: null, tag: null, index: {}, children: {} };

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

  function isEntry(e) { return !e.type || e.type === "entry"; }

  function load() {
    fetch("data.json", { cache: "no-store" })
      .then(function (r) { return r.json(); })
      .then(function (d) { state.data = d; render(); })
      .catch(function (e) { console.error("katra: failed to load data.json", e); });
  }

  function render() {
    var d = state.data;
    if (!d) return;
    var accent = d.site && d.site.accent;
    if (accent) document.documentElement.style.setProperty("--accent", accent);
    document.title = (d.site && d.site.title) || "Dev Log";
    setText("site-title", (d.site && d.site.title) || "Dev Log");
    setText("site-desc", (d.site && d.site.description) || "");

    var all = d.entries || [];
    state.index = {};
    state.children = {};
    all.forEach(function (e) {
      state.index[e.slug] = e;
      if (e.type === "task" && e.epic) {
        (state.children[e.epic] = state.children[e.epic] || []).push(e);
      }
    });

    var nodes = all.filter(function (e) {
      return !state.tag || (e.tags || []).indexOf(state.tag) >= 0;
    });

    // ── Classic entry zones (back-compat: type "entry" or empty) ──────────
    var entryNodes = nodes.filter(isEntry);
    var drafts = entryNodes.filter(function (e) { return e.draft; });
    var featured = entryNodes.filter(function (e) { return !e.draft && e.featured; });
    var log = entryNodes.filter(function (e) { return !e.draft && !e.featured; });
    fill("drafts", drafts);
    fill("featured", featured);
    fill("log", log);
    toggle("drafts-section", drafts.length > 0);
    toggle("featured-section", featured.length > 0);
    toggle("log-section", log.length > 0);

    // ── Board: tasks grouped by status ────────────────────────────────────
    var tasks = nodes.filter(function (e) { return e.type === "task"; });
    renderBoard(tasks);
    toggle("board-section", tasks.length > 0);

    // ── Roadmap: epics + loose tasks grouped by horizon ───────────────────
    var epics = nodes.filter(function (e) { return e.type === "epic"; });
    var looseTasks = tasks.filter(function (e) { return !e.epic; });
    renderRoadmap(epics, looseTasks);
    toggle("roadmap-section", epics.length > 0 || looseTasks.length > 0);

    // ── Decisions & Reference (articles) — full cards ─────────────────────
    var decisions = nodes.filter(function (e) { return e.type === "decision"; });
    var articles = nodes.filter(function (e) { return e.type === "article"; });
    fill("decisions", decisions);
    fill("articles", articles);
    toggle("decisions-section", decisions.length > 0);
    toggle("articles-section", articles.length > 0);

    document.getElementById("empty").hidden = nodes.length > 0;

    var entriesAll = all.filter(isEntry);
    setText("count-logged", entriesAll.filter(function (e) { return !e.draft; }).length + " logged");
    setText("count-drafts", entriesAll.filter(function (e) { return e.draft; }).length + " in progress");

    renderNav();
    renderTagbar();
    wireInteractions();
  }

  // ── In-page section nav (only sections that have content) ───────────────
  function renderNav() {
    var nav = document.getElementById("nav");
    if (!nav) return;
    nav.innerHTML = "";
    [
      ["board-section", "Board"],
      ["roadmap-section", "Roadmap"],
      ["drafts-section", "In Progress"],
      ["featured-section", "Deep Dives"],
      ["log-section", "The Log"],
      ["decisions-section", "Decisions"],
      ["articles-section", "Reference"],
    ].forEach(function (pair) {
      var sec = document.getElementById(pair[0]);
      if (!sec || sec.hidden) return;
      var a = el("a", "dl-nav-link", esc(pair[1]));
      a.href = "#" + pair[0];
      nav.appendChild(a);
    });
    nav.hidden = nav.childNodes.length === 0;
  }

  // ── Full card (entries, decisions, articles) ────────────────────────────
  function fill(id, list) {
    var box = document.getElementById(id);
    box.innerHTML = "";
    list.forEach(function (e) { box.appendChild(card(e)); });
  }

  function card(e) {
    var c = el("article", "dl-card" + (e.draft ? " is-draft" : ""));
    c.id = e.slug; // wikilink target: [[slug]] → <a href="#slug">

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

    // Node badges (type + status) for non-entry nodes; entries are unchanged.
    if (!isEntry(e)) {
      var badges = el("div", "dl-card-badges");
      badges.appendChild(el("span", "dl-badge dl-badge-type", esc(e.type)));
      if (e.status) badges.appendChild(el("span", "dl-badge dl-status-" + e.status, esc(e.status)));
      head.appendChild(badges);
    }

    if (e.tags && e.tags.length) {
      var tags = el("div", "dl-card-tags");
      e.tags.forEach(function (t) { tags.appendChild(el("span", "dl-chip", esc(t))); });
      head.appendChild(tags);
    }
    c.appendChild(head);

    c.appendChild(el("div", "dl-card-body", e.html || ""));
    var ft = foot(e);
    if (ft.childNodes.length) c.appendChild(ft);
    var bl = backlinksPanel(e);
    if (bl) c.appendChild(bl);
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

  // ── Backlinks panel — "what links here" ─────────────────────────────────
  function backlinksPanel(e) {
    if (!e.backlinks || !e.backlinks.length) return null;
    var p = el("div", "dl-backlinks");
    p.appendChild(el("div", "dl-backlinks-head", "Linked from"));
    var list = el("div", "dl-backlinks-list");
    e.backlinks.forEach(function (b) {
      var a = el("a", "dl-backlink");
      a.href = "#" + b.slug;
      a.appendChild(el("span", "dl-backlink-type", esc(b.type || "node")));
      a.appendChild(document.createTextNode(b.title || b.slug));
      list.appendChild(a);
    });
    p.appendChild(list);
    return p;
  }

  // ── Board (kanban by status) ────────────────────────────────────────────
  function renderBoard(tasks) {
    var box = document.getElementById("board");
    box.innerHTML = "";
    var cols = ["todo", "doing", "done"];
    if (tasks.some(function (t) { return t.status === "cut"; })) cols.push("cut");
    cols.forEach(function (status) {
      var colTasks = tasks.filter(function (t) { return (t.status || "todo") === status; });
      var col = el("div", "dl-col dl-col-" + status);
      var head = el("div", "dl-col-head");
      head.appendChild(el("span", "dl-col-name", esc(status)));
      head.appendChild(el("span", "dl-col-count", String(colTasks.length)));
      col.appendChild(head);
      var body = el("div", "dl-col-body");
      colTasks.forEach(function (t) { body.appendChild(taskCard(t, true)); });
      col.appendChild(body);
      box.appendChild(col);
    });
  }

  // A task node card. withId=true makes this the canonical anchor for the slug
  // (used on the board); roadmap re-uses tasks without an id to avoid dupes.
  function taskCard(t, withId) {
    var c = el("article", "dl-node dl-node-task");
    if (withId) c.id = t.slug;
    var title = el("a", "dl-node-title", esc(t.title));
    title.href = "#" + t.slug;
    c.appendChild(title);

    var meta = el("div", "dl-node-meta");
    if (t.effort) meta.appendChild(el("span", "dl-badge dl-badge-effort", esc(t.effort)));
    // On the roadmap the column is horizon, not status — show status there.
    if (!withId && t.status) meta.appendChild(el("span", "dl-badge dl-status-" + t.status, esc(t.status)));
    if (t.epic) {
      var ep = state.index[t.epic];
      var link = el("a", "dl-node-epic", "↳ " + esc(ep ? ep.title : t.epic));
      link.href = "#" + t.epic;
      meta.appendChild(link);
    }
    if (meta.childNodes.length) c.appendChild(meta);

    var bl = backlinksPanel(t);
    if (bl) c.appendChild(bl);
    return c;
  }

  // ── Roadmap (epics + loose tasks by horizon) ────────────────────────────
  function renderRoadmap(epics, looseTasks) {
    var box = document.getElementById("roadmap");
    box.innerHTML = "";
    var order = ["now", "next", "later"];
    var bucket = function (h) { return order.indexOf(h) >= 0 ? h : "unscheduled"; };
    var cols = order.slice();
    var hasUnscheduled = epics.concat(looseTasks).some(function (n) { return bucket(n.horizon) === "unscheduled"; });
    if (hasUnscheduled) cols.push("unscheduled");

    cols.forEach(function (h) {
      var colEpics = epics.filter(function (e) { return bucket(e.horizon) === h; });
      var colTasks = looseTasks.filter(function (t) { return bucket(t.horizon) === h; });
      var col = el("div", "dl-col dl-col-horizon dl-col-" + h);
      var head = el("div", "dl-col-head");
      head.appendChild(el("span", "dl-col-name", esc(h)));
      head.appendChild(el("span", "dl-col-count", String(colEpics.length + colTasks.length)));
      col.appendChild(head);
      var body = el("div", "dl-col-body");
      colEpics.forEach(function (e) { body.appendChild(epicCard(e)); });
      colTasks.forEach(function (t) { body.appendChild(taskCard(t, false)); });
      col.appendChild(body);
      box.appendChild(col);
    });
  }

  // An epic node card with a lightweight rollup of its child tasks.
  function epicCard(e) {
    var c = el("article", "dl-node dl-node-epic-card");
    c.id = e.slug;
    var title = el("a", "dl-node-title", esc(e.title));
    title.href = "#" + e.slug;
    c.appendChild(title);

    var kids = state.children[e.slug] || [];
    var meta = el("div", "dl-node-meta");
    if (e.status) meta.appendChild(el("span", "dl-badge dl-status-" + e.status, esc(e.status)));
    if (kids.length) meta.appendChild(el("span", "dl-badge", kids.length + " task" + (kids.length > 1 ? "s" : "")));
    if (meta.childNodes.length) c.appendChild(meta);

    if (kids.length) {
      var ul = el("ul", "dl-epic-tasks");
      kids.forEach(function (k) {
        var li = el("li");
        var a = el("a", "dl-epic-task-link dl-status-" + (k.status || "todo"));
        a.href = "#" + k.slug;
        a.appendChild(el("span", "dl-epic-task-mark"));
        a.appendChild(document.createTextNode(k.title));
        li.appendChild(a);
        ul.appendChild(li);
      });
      c.appendChild(ul);
    }

    var bl = backlinksPanel(e);
    if (bl) c.appendChild(bl);
    return c;
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
