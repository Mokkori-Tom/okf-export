// okf-export page behaviour: copy buttons on code blocks, the file header
// buttons (Preview/Raw, copy, outline) and search. Shared by the multi-page
// site and the single-file export, where every page is in one document.
(function () {
  "use strict";
  var COPY = '<svg class="octicon" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"/><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"/></svg>';
  var CHECK = '<svg class="octicon" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"/></svg>';

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

  // Images given only a height ({height=800px}) are capped at the page width
  // by max-width, which would squash them on a narrow screen. Shrink the
  // height too so they keep their shape.
  var tall = [];
  document.querySelectorAll(".markdown-body img[style]").forEach(function (img) {
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
  function fitImages() { tall.forEach(fitImage); }
  var fitting = false;
  window.addEventListener("resize", function () {
    if (fitting) return;
    fitting = true;
    requestAnimationFrame(function () { fitting = false; fitImages(); });
  });
  window.addEventListener("hashchange", function () { setTimeout(fitImages); }); // single-file page switch
  fitImages();

  // Image pages: show the size in pixels next to the file size.
  document.querySelectorAll(".image-view img").forEach(function (img) {
    function dims() {
      var c = img.closest(".okf-file").querySelector(".img-dims");
      if (!c || !img.naturalWidth) return;
      c.textContent = img.naturalWidth + " × " + img.naturalHeight;
      c.hidden = false;
    }
    if (img.complete) dims(); else img.addEventListener("load", dims);
  });

  // PDF pages: open the embedded file in the browser's PDF viewer. A blob:
  // URL works where a data: URL is refused. Browsers without a viewer
  // (most phones) keep the note and the download button. In the single-file
  // export only the page being shown is loaded.
  function showPDFs() {
    if (navigator.pdfViewerEnabled === false || !window.fetch) return;
    document.querySelectorAll(".pdf-view:not(.loaded)").forEach(function (v) {
      var page = v.closest(".okf-page"), a = v.querySelector("a[download]");
      if ((page && page.hidden) || !a || !a.href) return;
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
  window.addEventListener("hashchange", function () { setTimeout(showPDFs); });
  showPDFs();

  // Copy buttons on code blocks.
  document.querySelectorAll(".markdown-body.rendered pre").forEach(function (pre) {
    var wrap = document.createElement("div");
    wrap.className = "code-wrap";
    pre.parentNode.insertBefore(wrap, pre);
    wrap.appendChild(pre);
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "btn btn-icon code-copy";
    btn.title = "Copy";
    btn.setAttribute("aria-label", "Copy");
    btn.innerHTML = COPY;
    btn.addEventListener("click", function () { copyText(pre.textContent, btn); });
    wrap.appendChild(btn);
  });

  function rawText(box) {
    var cells = box.querySelectorAll(".raw-view .lntd pre");
    if (cells.length) return cells[cells.length - 1].textContent;
    var pre = box.querySelector(".raw-view pre");
    return pre ? pre.textContent : "";
  }

  function showRaw(box, raw) {
    var r = box.querySelector(".rendered"), v = box.querySelector(".raw-view"), o = box.querySelector(".outline");
    if (r) r.hidden = raw;
    if (v) v.hidden = !raw;
    if (o && raw) closeOutline(box);
    box.querySelectorAll("[data-okf=preview],[data-okf=raw]").forEach(function (b) {
      b.classList.toggle("selected", (b.dataset.okf === "raw") === raw);
    });
  }

  function closeOutline(box) {
    var o = box.querySelector(".outline"), b = box.querySelector("[data-okf=outline]");
    if (o) o.hidden = true;
    if (b) b.setAttribute("aria-expanded", "false");
  }

  function toggleOutline(box, btn) {
    var o = box.querySelector(".outline");
    if (!o) return;
    if (!o.hidden) { closeOutline(box); return; }
    showRaw(box, false);
    if (!o.dataset.built) {
      o.dataset.built = "1";
      var hs = box.querySelectorAll(".rendered h1[id], .rendered h2[id], .rendered h3[id], .rendered h4[id], .rendered h5[id], .rendered h6[id]");
      if (!hs.length) {
        o.textContent = "No headings";
      }
      hs.forEach(function (h) {
        var a = document.createElement("a");
        a.href = "#" + h.id;
        a.textContent = h.textContent;
        a.className = "outline-" + h.tagName.toLowerCase();
        o.appendChild(a);
      });
    }
    o.hidden = false;
    btn.setAttribute("aria-expanded", "true");
  }

  document.addEventListener("click", function (ev) {
    var btn = ev.target.closest && ev.target.closest("[data-okf]");
    if (!btn) return;
    var box = btn.closest(".okf-file");
    switch (btn.dataset.okf) {
      case "preview": showRaw(box, false); break;
      case "raw": showRaw(box, true); break;
      case "copy": copyText(rawText(box), btn); break;
      case "outline": toggleOutline(box, btn); break;
    }
  });

  // Search over every page's title, path, description and text.
  var input = document.querySelector(".search-input");
  var results = document.querySelector(".search-results");
  if (!input || !results) return;
  var embedded = document.getElementById("okf-search");
  var root = document.body.dataset.root || "";
  var index = null, loading = false, sel = 0;

  function load(cb) {
    if (index) return cb();
    if (embedded) { index = JSON.parse(embedded.textContent || "[]"); return cb(); }
    if (loading) return;
    loading = true;
    var s = document.createElement("script");
    s.src = root + "okf-search.js";
    s.onload = s.onerror = function () { index = window.OKF_SEARCH || []; cb(); };
    document.head.appendChild(s);
  }

  function href(page) { return embedded ? "#/" + page : root + page; }

  function highlight(el, text, terms) {
    var low = text.toLowerCase(), i = 0;
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

  function snippet(e, terms) {
    var text = e.x || e.d || "", low = text.toLowerCase(), at = -1;
    terms.forEach(function (t) { var j = low.indexOf(t); if (j >= 0 && (at < 0 || j < at)) at = j; });
    if (at < 0) return text.slice(0, 120);
    var start = Math.max(0, at - 40);
    return (start ? "…" : "") + text.slice(start, at + 100) + (at + 100 < text.length ? "…" : "");
  }

  function run() {
    var q = input.value.trim().toLowerCase();
    results.textContent = "";
    sel = 0;
    if (!q) { results.hidden = true; return; }
    var terms = q.split(/\s+/);
    var hits = [];
    index.forEach(function (e) {
      if (!e._t) { e._t = (e.t || "").toLowerCase(); e._h = [e._t, e.p, e.d || "", e.x || ""].join("\n").toLowerCase(); }
      var score = 0;
      for (var k = 0; k < terms.length; k++) {
        if (e._h.indexOf(terms[k]) < 0) return;
        if (e._t.indexOf(terms[k]) >= 0) score += 10;
        else if (e.p.toLowerCase().indexOf(terms[k]) >= 0) score += 5;
        else if ((e.d || "").toLowerCase().indexOf(terms[k]) >= 0) score += 3;
        else score += 1;
      }
      hits.push({ e: e, s: score });
    });
    hits.sort(function (a, b) { return b.s - a.s || (a.e.t < b.e.t ? -1 : a.e.t > b.e.t ? 1 : 0); });
    if (!hits.length) {
      var none = document.createElement("div");
      none.className = "search-none";
      none.textContent = "No results";
      results.appendChild(none);
    }
    hits.slice(0, 30).forEach(function (h, i) {
      var a = document.createElement("a");
      a.className = "search-hit" + (i === 0 ? " selected" : "");
      a.href = href(h.e.u);
      var t = document.createElement("span"); t.className = "hit-title"; highlight(t, h.e.t || h.e.u, terms);
      var p = document.createElement("span"); p.className = "hit-path"; p.textContent = h.e.p;
      var x = document.createElement("span"); x.className = "hit-text"; highlight(x, snippet(h.e, terms), terms);
      a.appendChild(t); a.appendChild(p); a.appendChild(x);
      results.appendChild(a);
    });
    results.hidden = false;
  }

  function close() { results.hidden = true; }

  input.addEventListener("input", function () { load(run); });
  input.addEventListener("focus", function () { if (input.value.trim()) load(run); });
  input.addEventListener("keydown", function (ev) {
    var hits = results.querySelectorAll(".search-hit");
    if (ev.key === "Escape") { close(); input.blur(); return; }
    if (!hits.length) return;
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      hits[sel].classList.remove("selected");
      sel = (sel + (ev.key === "ArrowDown" ? 1 : hits.length - 1)) % hits.length;
      hits[sel].classList.add("selected");
      hits[sel].scrollIntoView({ block: "nearest" });
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      hits[sel].click();
    }
  });
  results.addEventListener("click", function (ev) {
    if (ev.target.closest(".search-hit")) { close(); input.blur(); }
  });
  document.addEventListener("click", function (ev) {
    if (!ev.target.closest(".search")) close();
  });
  document.addEventListener("keydown", function (ev) {
    var t = ev.target.tagName;
    if (ev.key === "/" && t !== "INPUT" && t !== "TEXTAREA" && !ev.target.isContentEditable) {
      ev.preventDefault();
      input.focus();
      input.select();
    }
  });
})();
