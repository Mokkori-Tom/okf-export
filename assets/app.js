// okf-export page behaviour, after mdBook: themes, the sidebar, chapter
// arrows and keys, search, and the page tools (source view, copy, outline).
// The same script runs in the multi-page site and in the single-file export,
// where every page is a hidden ".okf-page" switched by the router, which
// sends an "okf:page" event when the page changes.
(function () {
  "use strict";
  var doc = document.documentElement;
  var single = !!document.getElementById("okf-search");
  var root = doc.dataset.root || "";

  function store(k, v) { try { if (v === null) localStorage.removeItem(k); else localStorage.setItem(k, v); } catch (e) {} }
  function href(page) { return single ? "#/" + page : root + page; }
  // The page being shown: the whole document, or the visible ".okf-page".
  function scope() { return single ? document.querySelector(".okf-page:not([hidden])") || document : document; }
  function typing(el) { return el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.isContentEditable); }

  var COPY = '<svg class="icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"/><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"/></svg>';
  var CHECK = '<svg class="icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"/></svg>';

  function copyText(text, btn) {
    function done() {
      if (btn.dataset.busy) return;
      btn.dataset.busy = "1";
      var html = btn.innerHTML;
      btn.innerHTML = CHECK;
      btn.classList.add("copied");
      setTimeout(function () { btn.innerHTML = html; btn.classList.remove("copied"); delete btn.dataset.busy; }, 1500);
    }
    function fallback() {
      // navigator.clipboard needs a secure context, which a LAN preview isn't.
      var ta = document.createElement("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.style.position = "fixed";
      ta.style.top = "-1000px";
      document.body.appendChild(ta);
      ta.select();
      try { if (document.execCommand("copy")) done(); } catch (e) {}
      document.body.removeChild(ta);
    }
    if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(text).then(done, fallback);
    else fallback();
  }

  // ---- Themes ----
  var themeToggle = document.getElementById("theme-toggle");
  var themeList = document.getElementById("theme-list");
  function currentTheme() { return (doc.className.match(/\b(light|rust|coal|navy|ayu)\b/) || ["light"])[0]; }
  function markTheme() {
    themeList.querySelectorAll(".theme").forEach(function (b) { b.classList.toggle("selected", b.dataset.theme === currentTheme()); });
  }
  function setTheme(t) {
    doc.classList.remove("light", "rust", "coal", "navy", "ayu");
    doc.classList.add(t);
    store("okf-theme", t);
    markTheme();
  }
  function showThemes(show) {
    themeList.hidden = !show;
    themeToggle.setAttribute("aria-expanded", String(show));
    if (show) { markTheme(); (themeList.querySelector(".selected") || themeList.querySelector(".theme")).focus(); }
  }
  themeToggle.addEventListener("click", function () { showThemes(themeList.hidden); });
  themeList.addEventListener("click", function (ev) {
    var b = ev.target.closest(".theme");
    if (b) { setTheme(b.dataset.theme); showThemes(false); themeToggle.focus(); }
  });

  // ---- Sidebar ----
  var sidebar = document.getElementById("sidebar");
  var toc = document.getElementById("toc");
  function narrow() { return window.innerWidth <= 620; }
  function showSidebar(show) {
    doc.classList.toggle("sidebar-visible", show);
    doc.classList.toggle("sidebar-hidden", !show);
    if (!narrow()) store("okf-sidebar", show ? "visible" : "hidden");
  }
  document.getElementById("sidebar-toggle").addEventListener("click", function () {
    showSidebar(!doc.classList.contains("sidebar-visible"));
  });
  // On a phone the sidebar covers the page: close it after choosing a page,
  // or when the page is tapped.
  sidebar.addEventListener("click", function (ev) { if (ev.target.closest("a[data-path]") && narrow()) showSidebar(false); });
  document.getElementById("page-wrapper").addEventListener("click", function (ev) {
    if (narrow() && doc.classList.contains("sidebar-visible") && !ev.target.closest("#sidebar-toggle")) showSidebar(false);
  });

  // Drag the sidebar's edge to resize it.
  sidebar.querySelector(".sidebar-resize-handle").addEventListener("pointerdown", function (ev) {
    ev.preventDefault();
    doc.classList.add("sidebar-resizing");
    function move(e) {
      doc.style.setProperty("--sidebar-width", Math.max(150, Math.min(e.clientX, window.innerWidth * 0.8)) + "px");
    }
    function up() {
      doc.classList.remove("sidebar-resizing");
      store("okf-sidebar-width", doc.style.getPropertyValue("--sidebar-width"));
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    }
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  });

  // The table of contents: shared by all pages in okf-toc.js, or already in
  // the single file. Links get their URL here and the current page is marked.
  if (!toc.firstElementChild && window.OKF_TOC) toc.innerHTML = window.OKF_TOC;
  toc.querySelectorAll("a[data-path]").forEach(function (a) { a.href = href(a.dataset.path); });
  toc.addEventListener("click", function (ev) {
    var t = ev.target.closest(".toggle");
    if (t) t.closest(".chapter-item").classList.toggle("expanded");
  });
  function markActive(scrollIntoView) {
    var page = doc.dataset.page || "index.html";
    toc.querySelectorAll("a.active").forEach(function (a) { a.classList.remove("active"); a.removeAttribute("aria-current"); });
    var a = toc.querySelector('a[data-path="' + page.replace(/["\\]/g, "\\$&") + '"]');
    if (!a) return;
    a.classList.add("active");
    a.setAttribute("aria-current", "page");
    for (var li = a.closest(".chapter-item"); li; li = li.parentElement.closest(".chapter-item")) li.classList.add("expanded");
    if (scrollIntoView) {
      var box = toc.getBoundingClientRect(), r = a.getBoundingClientRect();
      if (r.top < box.top || r.bottom > box.bottom) toc.scrollTop += r.top - box.top - box.height / 3;
    }
  }
  // Keep the sidebar's scroll position from page to page, like mdBook.
  var saved = null;
  try { saved = sessionStorage.getItem("okf-toc-scroll"); } catch (e) {}
  if (!single && saved !== null) toc.scrollTop = +saved;
  markActive(single || saved === null);
  window.addEventListener("pagehide", function () { try { sessionStorage.setItem("okf-toc-scroll", String(toc.scrollTop)); } catch (e) {} });

  // ---- Menu bar, print, keys ----
  var menuBar = document.getElementById("menu-bar");
  window.addEventListener("scroll", function () { menuBar.classList.toggle("bordered", window.scrollY > 0); }, { passive: true });
  // Print the whole book from print.html, as mdBook does; the single file
  // prints the page being shown.
  document.getElementById("print-button").addEventListener("click", function () {
    if (single) window.print(); else location.href = root + "print.html?print";
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.altKey || ev.ctrlKey || ev.metaKey || typing(ev.target)) return;
    if (ev.key === "ArrowLeft" || ev.key === "ArrowRight") {
      if (ev.shiftKey) return;
      var a = scope().querySelector(ev.key === "ArrowLeft" ? "a[rel=prev]" : "a[rel=next]");
      if (a) { ev.preventDefault(); a.click(); }
    } else if (ev.key === "/" || ev.key === "s" || ev.key === "S") {
      ev.preventDefault();
      showSearch(true);
    } else if ((ev.key === "o" || ev.key === "O") && !outlineToggle.hidden) {
      ev.preventDefault();
      showOutline(outline.hidden);
    } else if (ev.key === "Escape") {
      showThemes(false);
      showOutline(false);
      unmark();
    }
  });
  document.addEventListener("click", function (ev) {
    if (!themeList.hidden && !ev.target.closest("#theme-list, #theme-toggle")) showThemes(false);
    if (!outline.hidden && !ev.target.closest("#outline, #outline-toggle")) showOutline(false);
  });

  // ---- Page tools: source view, copy, outline ----
  function rawText(box) {
    var cells = box.querySelectorAll(".raw-view .lntd pre");
    if (cells.length) return cells[cells.length - 1].textContent;
    var pre = box.querySelector(".raw-view pre");
    return pre ? pre.textContent : "";
  }
  function showRaw(box, raw) {
    box.querySelectorAll(".okf-file > .rendered").forEach(function (r) { r.hidden = raw; });
    box.querySelectorAll(".okf-file > .raw-view[hidden], .okf-file > .raw-view.toggled").forEach(function (v) {
      v.hidden = !raw;
      v.classList.add("toggled");
    });
    var b = box.querySelector("[data-okf=raw]");
    if (b) { b.classList.toggle("selected", raw); b.setAttribute("aria-pressed", String(raw)); b.title = raw ? "Show the page" : "Show the source"; }
  }
  // The outline of the page's headings opens from the menu bar, which stays
  // on screen, so it can be used anywhere on the page. The heading being
  // read is marked.
  var outlineToggle = document.getElementById("outline-toggle");
  var outline = document.getElementById("outline");
  function headings() {
    return scope().querySelectorAll(".rendered :is(h1, h2, h3, h4, h5, h6)[id], .okf-header h1[id]");
  }
  function updateOutlineButton() { outlineToggle.hidden = !headings().length; showOutline(false); }
  function showOutline(show) {
    outline.hidden = !show;
    outlineToggle.setAttribute("aria-expanded", String(show));
    if (!show) return;
    showRaw(scope(), false);
    outline.textContent = "";
    var hs = Array.prototype.slice.call(headings()), current = null;
    hs.forEach(function (h) {
      if (h.getBoundingClientRect().top <= menuBar.offsetHeight + 20) current = h;
    });
    hs.forEach(function (h) {
      var a = document.createElement("a");
      a.href = "#" + h.id;
      a.textContent = h.textContent;
      a.className = "outline-" + h.tagName.toLowerCase() + (h === (current || hs[0]) ? " current" : "");
      a.addEventListener("click", function () { showOutline(false); });
      outline.appendChild(a);
    });
    var cur = outline.querySelector(".current");
    if (cur) outline.scrollTop = cur.offsetTop - outline.clientHeight / 3;
  }
  outlineToggle.addEventListener("click", function () { showOutline(outline.hidden); });
  document.addEventListener("click", function (ev) {
    var btn = ev.target.closest("[data-okf]");
    if (!btn) return;
    var box = btn.closest(".okf-page, main");
    switch (btn.dataset.okf) {
      case "raw": showRaw(box, btn.getAttribute("aria-pressed") !== "true"); break;
      case "copy": copyText(rawText(box), btn); break;
      case "save": savePage(box); break;
    }
  });

  // Save the page being read as one HTML file: its text with the site's
  // stylesheet and the current theme, images included, without the sidebar
  // and scripts. Links to other pages point back at the site.
  function savePage(box) {
    var main = box.matches("main") ? box : box.querySelector("main");
    var clone = main.cloneNode(true), orig = main.querySelectorAll("a[href]");
    clone.querySelectorAll("a[href]").forEach(function (a, i) {
      var h = a.getAttribute("href");
      if (h.charAt(0) === "#" && h.charAt(1) !== "/") return; // within the page
      if (!/^(data|blob):/.test(h)) a.setAttribute("href", orig[i].href);
    });
    clone.querySelectorAll(".page-tools, .code-copy, script").forEach(function (e) { e.remove(); });
    clone.querySelectorAll("mark.search-hit").forEach(function (m) { m.replaceWith(document.createTextNode(m.textContent)); });
    var holder = main.closest(".okf-page"), title = holder ? holder.dataset.title : document.title;
    var page = doc.dataset.page || "index.html";
    var html = "<!DOCTYPE html>\n<html lang=\"" + doc.lang + "\" class=\"" + currentTheme() + "\">\n<head>\n<meta charset=\"utf-8\">\n" +
      "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>" + title.replace(/&/g, "&amp;").replace(/</g, "&lt;") + "</title>\n" +
      "<style>" + document.querySelector("style").textContent + "</style>\n</head>\n<body>\n<div class=\"content\"><main>" +
      clone.innerHTML + "</main></div>\n</body>\n</html>\n";
    var a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([html], { type: "text/html" }));
    a.download = page.split("/").pop();
    document.body.appendChild(a);
    a.click();
    setTimeout(function () { URL.revokeObjectURL(a.href); a.remove(); }, 1000);
  }

  // Copy buttons on code blocks, and links on headings, as in mdBook.
  document.querySelectorAll(".content .rendered pre").forEach(function (pre) {
    var wrap = document.createElement("div");
    wrap.className = "code-wrap";
    pre.parentNode.insertBefore(wrap, pre);
    wrap.appendChild(pre);
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "code-copy";
    btn.title = "Copy to clipboard";
    btn.setAttribute("aria-label", "Copy to clipboard");
    btn.innerHTML = COPY;
    btn.addEventListener("click", function () { copyText(pre.textContent, btn); });
    wrap.appendChild(btn);
  });
  document.querySelectorAll(".content .rendered :is(h1, h2, h3, h4, h5, h6)[id]").forEach(function (h) {
    if (h.querySelector("a")) return;
    var a = document.createElement("a");
    a.className = "header-anchor";
    a.href = "#" + h.id;
    while (h.firstChild) a.appendChild(h.firstChild);
    h.appendChild(a);
  });

  // Images given only a height ({height=800px}) are capped at the page width
  // by max-width, which would squash them on a narrow screen. Shrink the
  // height too so they keep their shape.
  var tall = [];
  document.querySelectorAll(".content img[style]").forEach(function (img) {
    if (img.style.height && !img.style.width) {
      img.dataset.height = img.style.height;
      tall.push(img);
      img.addEventListener("load", function () { fitImage(img); });
    }
  });
  function fitImage(img) {
    var box = img.parentElement, cs = getComputedStyle(box);
    var avail = box.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    if (!img.naturalWidth || !img.naturalHeight || avail <= 0) return; // not loaded, or on a hidden page
    img.style.height = img.dataset.height;
    var ratio = img.naturalWidth / img.naturalHeight;
    if (img.offsetHeight * ratio > avail) img.style.height = avail / ratio + "px";
  }
  var fitting = false;
  window.addEventListener("resize", function () {
    if (fitting) return;
    fitting = true;
    requestAnimationFrame(function () { fitting = false; tall.forEach(fitImage); });
  });

  // Image pages: show the size in pixels.
  document.querySelectorAll(".image-view img").forEach(function (img) {
    function dims() {
      var c = img.closest(".okf-file").querySelector(".img-dims");
      if (c && img.naturalWidth) c.textContent = "· " + img.naturalWidth + " × " + img.naturalHeight;
    }
    if (img.complete) dims(); else img.addEventListener("load", dims);
  });

  // PDF pages: open the embedded file in the browser's PDF viewer. A blob:
  // URL works where a data: URL is refused. Browsers without a viewer
  // (most phones) keep the note and the download button.
  function showPDFs() {
    if (navigator.pdfViewerEnabled === false || !window.fetch) return;
    scope().querySelectorAll(".pdf-view:not(.loaded)").forEach(function (v) {
      var a = v.querySelector("a[download]");
      if (!a || !a.href) return;
      v.classList.add("loaded");
      fetch(a.href).then(function (r) { return r.arrayBuffer(); }).then(function (buf) {
        var f = v.querySelector("iframe");
        f.src = URL.createObjectURL(new Blob([buf], { type: "application/pdf" }));
        f.hidden = false;
        v.querySelector(".media-note").hidden = true;
        a.hidden = true;
      }, function () { v.classList.remove("loaded"); });
    });
  }

  // ---- Search ----
  var searchToggle = document.getElementById("search-toggle");
  var wrapper = document.getElementById("search-wrapper");
  var bar = document.getElementById("searchbar");
  var outer = document.getElementById("searchresults-outer");
  var header = document.getElementById("searchresults-header");
  var list = document.getElementById("searchresults");
  var index = null, loading = false, timer = 0;

  // Text is compared NFKC-normalised and lower-cased, so full-width and
  // half-width letters, digits and kana match each other. Matching is by
  // substring, so Japanese and Chinese need no word splitting.
  function norm(s) { return (s || "").normalize("NFKC").toLowerCase(); }

  function showSearch(show) {
    wrapper.hidden = !show;
    searchToggle.setAttribute("aria-expanded", String(show));
    if (show) { bar.focus(); bar.select(); if (bar.value.trim()) load(run); }
  }
  searchToggle.addEventListener("click", function () { showSearch(wrapper.hidden); });

  function load(cb) {
    if (index) return cb();
    var embedded = document.getElementById("okf-search");
    if (embedded) { index = JSON.parse(embedded.textContent || "[]"); return cb(); }
    if (loading) return;
    loading = true;
    var s = document.createElement("script");
    s.src = root + "okf-search.js";
    s.onload = s.onerror = function () { index = window.OKF_SEARCH || []; cb(); };
    document.head.appendChild(s);
  }

  function markText(el, text, terms) {
    var low = norm(text), i = 0;
    // NFKC can change the length; then there is nothing safe to mark.
    if (low.length !== text.length) { el.appendChild(document.createTextNode(text)); return; }
    while (i < text.length) {
      var best = -1, len = 0;
      terms.forEach(function (t) {
        var j = low.indexOf(t, i);
        if (j >= 0 && (best < 0 || j < best)) { best = j; len = t.length; }
      });
      if (best < 0) break;
      el.appendChild(document.createTextNode(text.slice(i, best)));
      var m = document.createElement("mark");
      m.textContent = text.slice(best, best + len);
      el.appendChild(m);
      i = best + len;
    }
    el.appendChild(document.createTextNode(text.slice(i)));
  }

  function teaser(e, terms) {
    var text = e.x || "", low = norm(text), at = -1;
    terms.forEach(function (t) { var j = low.indexOf(t); if (j >= 0 && (at < 0 || j < at)) at = j; });
    if (at < 0) return text.slice(0, 160) + (text.length > 160 ? "…" : "");
    var start = Math.max(0, at - 60);
    return (start ? "…" : "") + text.slice(start, at + 120) + (at + 120 < text.length ? "…" : "");
  }

  function count(hay, t) {
    var n = 0, i = hay.indexOf(t);
    while (i >= 0 && n < 10) { n++; i = hay.indexOf(t, i + t.length); }
    return n;
  }

  function run() {
    var q = bar.value.trim();
    list.textContent = "";
    if (!q) { outer.hidden = true; return; }
    var terms = norm(q).split(/\s+/).filter(Boolean), hits = [];
    index.forEach(function (e) {
      if (e._x === undefined) { e._h = norm(e.t + " " + (e.h || "")); e._p = norm(e.p); e._x = norm(e.x); }
      var score = 0;
      for (var k = 0; k < terms.length; k++) {
        var t = terms[k], inHead = e._h.indexOf(t) >= 0, inPath = e._p.indexOf(t) >= 0, n = count(e._x, t);
        if (!inHead && !inPath && !n) return;
        score += (inHead ? 20 : 0) + (inPath ? 5 : 0) + n;
      }
      hits.push({ e: e, s: score });
    });
    hits.sort(function (a, b) { return b.s - a.s; });
    header.textContent = hits.length
      ? hits.length + " search result" + (hits.length === 1 ? "" : "s") + " for '" + q + "':"
      : "No search results for '" + q + "'.";
    hits.slice(0, 30).forEach(function (h) {
      var e = h.e, li = document.createElement("li"), a = document.createElement("a");
      a.className = "breadcrumbs";
      a.href = href(e.u) + (e.a ? "#" + e.a : "");
      markText(a, e.t + (e.h ? " » " + e.h : ""), terms);
      var where = document.createElement("span");
      where.className = "where";
      where.textContent = e.p;
      a.appendChild(where);
      var x = document.createElement("span");
      x.className = "teaser";
      markText(x, teaser(e, terms), terms);
      li.appendChild(a);
      li.appendChild(x);
      list.appendChild(li);
    });
    outer.hidden = false;
  }

  bar.addEventListener("input", function () {
    clearTimeout(timer);
    timer = setTimeout(function () { load(run); }, 120);
  });
  // Arrow keys move through the results and Enter opens one, as in mdBook.
  wrapper.addEventListener("keydown", function (ev) {
    var items = Array.prototype.slice.call(list.children), cur = list.querySelector("li.focus"), i = items.indexOf(cur);
    if (ev.key === "Escape") {
      ev.preventDefault();
      showSearch(false);
    } else if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      if (!items.length) return;
      ev.preventDefault();
      if (cur) cur.classList.remove("focus");
      i = ev.key === "ArrowDown" ? Math.min(i + 1, items.length - 1) : i - 1;
      if (i < 0) return;
      items[i].classList.add("focus");
      items[i].scrollIntoView({ block: "nearest" });
    } else if (ev.key === "Enter" && items.length) {
      ev.preventDefault();
      (cur || items[0]).querySelector("a").click();
    }
  });
  list.addEventListener("click", function (ev) {
    if (!ev.target.closest("a")) return;
    try { sessionStorage.setItem("okf-highlight", bar.value.trim()); } catch (e) {}
    if (single) { showSearch(false); setTimeout(highlight); }
  });

  // Mark the searched words on the page a result led to; Esc removes them.
  function highlight() {
    var q = null;
    try { q = sessionStorage.getItem("okf-highlight"); sessionStorage.removeItem("okf-highlight"); } catch (e) {}
    if (!q) return;
    unmark();
    var terms = norm(q).split(/\s+/).filter(Boolean);
    scope().querySelectorAll(".rendered").forEach(function (r) {
      var walker = document.createTreeWalker(r, NodeFilter.SHOW_TEXT), nodes = [];
      while (walker.nextNode()) if (!walker.currentNode.parentElement.closest("pre, script, style")) nodes.push(walker.currentNode);
      nodes.forEach(function (n) {
        var low = norm(n.data);
        if (low.length !== n.data.length || !terms.some(function (t) { return low.indexOf(t) >= 0; })) return;
        var frag = document.createDocumentFragment(), span = document.createElement("span");
        markText(span, n.data, terms);
        span.querySelectorAll("mark").forEach(function (m) { m.className = "search-hit"; });
        while (span.firstChild) frag.appendChild(span.firstChild);
        n.parentNode.replaceChild(frag, n);
      });
    });
  }
  function unmark() {
    document.querySelectorAll("mark.search-hit").forEach(function (m) {
      var p = m.parentNode;
      p.replaceChild(document.createTextNode(m.textContent), m);
      p.normalize();
    });
  }

  // ---- Page shown ----
  // A result on the page being read only changes the hash.
  if (!single) window.addEventListener("hashchange", highlight);
  document.addEventListener("okf:page", function () {
    markActive(true);
    updateOutlineButton();
    showPDFs();
    tall.forEach(fitImage);
    highlight();
  });
  updateOutlineButton();
  showPDFs();
  tall.forEach(fitImage);
  highlight();
})();
