(function () {
  "use strict";

  /* ---------------------------------------------------------------
     Nav epoch clock — real epoch seconds, ticking every second.
     Mirrors the product's own "every report carries an epoch
     timestamp" detail.
     --------------------------------------------------------------- */
  var epochEl = document.getElementById("epoch-clock");
  function tickClock() {
    if (epochEl) {
      epochEl.textContent = String(Math.floor(Date.now() / 1000));
    }
  }
  tickClock();
  setInterval(tickClock, 1000);

  /* ---------------------------------------------------------------
     Footer "this page" status line — a small, honest wink at the
     product: the page itself reports its own uptime.
     --------------------------------------------------------------- */
  var footerStatus = document.getElementById("footer-status");
  var loadedAt = Date.now();
  function pad(n) {
    return String(n).padStart(2, "0");
  }
  function tickFooter() {
    if (!footerStatus) return;
    var elapsed = Math.floor((Date.now() - loadedAt) / 1000);
    var h = Math.floor(elapsed / 3600);
    var m = Math.floor((elapsed % 3600) / 60);
    var s = elapsed % 60;
    footerStatus.textContent =
      "manager: active · this page has been up for " + pad(h) + ":" + pad(m) + ":" + pad(s);
  }
  tickFooter();
  setInterval(tickFooter, 1000);

  /* ---------------------------------------------------------------
     Example fleet board — simulates independent worker reporting
     intervals. Each row has its own configured interval; when it
     elapses, that row (and only that row) gets a fresh reading,
     the way real workers report on their own schedule rather than
     in lockstep.
     --------------------------------------------------------------- */
  var rows = document.querySelectorAll("#board-body tr");
  if (rows.length) {
    var state = Array.prototype.map.call(rows, function (row) {
      var pingCell = row.querySelector(".c-ping");
      var diskCell = row.querySelector(".c-disk");
      var cpuCell = row.querySelector(".c-cpu");
      return {
        row: row,
        interval: parseInt(row.getAttribute("data-interval"), 10) || 20,
        secsAgo: Math.floor(Math.random() * 6),
        ping: parseInt(pingCell.textContent, 10) || 10,
        disk: parseInt(diskCell.textContent, 10) || 30,
        cpu: parseInt(cpuCell.textContent, 10) || 10,
        pingCell: pingCell,
        diskCell: diskCell,
        cpuCell: cpuCell,
        seenCell: row.querySelector(".c-seen"),
        dot: row.querySelector(".c-host .dot"),
      };
    });

    function clamp(v, min, max) {
      return Math.max(min, Math.min(max, v));
    }

    function classify(cell, value, warnAt, failAt) {
      cell.classList.remove("warn", "fail");
      if (value >= failAt) cell.classList.add("fail");
      else if (value >= warnAt) cell.classList.add("warn");
    }

    function refreshRow(s) {
      s.ping = clamp(s.ping + (Math.random() * 6 - 3), 3, 120);
      s.cpu = clamp(s.cpu + (Math.random() * 10 - 5), 1, 98);
      s.disk = clamp(s.disk + (Math.random() * 2 - 1), 4, 96);

      s.pingCell.textContent = Math.round(s.ping) + "ms";
      s.cpuCell.textContent = Math.round(s.cpu) + "%";
      s.diskCell.textContent = Math.round(s.disk) + "%";

      classify(s.cpuCell, s.cpu, 60, 85);
      classify(s.diskCell, s.disk, 65, 90);

      var gitFail = s.row.querySelector(".c-git").classList.contains("fail");
      var anyWarn = s.cpuCell.classList.contains("warn") || s.diskCell.classList.contains("warn");
      var anyFail = s.cpuCell.classList.contains("fail") || s.diskCell.classList.contains("fail");

      s.dot.classList.remove("dot-ok", "dot-warn", "dot-fail");
      if (gitFail || anyFail) s.dot.classList.add("dot-fail");
      else if (anyWarn) s.dot.classList.add("dot-warn");
      else s.dot.classList.add("dot-ok");

      s.secsAgo = 0;
    }

    function tickBoard() {
      state.forEach(function (s) {
        s.secsAgo += 1;
        if (s.secsAgo >= s.interval) {
          refreshRow(s);
        }
        s.seenCell.textContent = s.secsAgo + "s ago";
      });
    }

    setInterval(tickBoard, 1000);
  }

  /* ---------------------------------------------------------------
     Scroll reveal for section headings — a single orchestrated
     entrance per section, not a scattered animate-everything pass.
     --------------------------------------------------------------- */
  var prefersReducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var revealEls = document.querySelectorAll(".reveal");

  if (revealEls.length && !prefersReducedMotion && "IntersectionObserver" in window) {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry, i) {
          if (entry.isIntersecting) {
            setTimeout(function () {
              entry.target.classList.add("in-view");
            }, i * 60);
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.2, rootMargin: "0px 0px -40px 0px" }
    );
    revealEls.forEach(function (el) {
      observer.observe(el);
    });
  } else {
    revealEls.forEach(function (el) {
      el.classList.add("in-view");
    });
  }

  /* ---------------------------------------------------------------
     Copy-to-clipboard for install snippets.
     --------------------------------------------------------------- */
  document.querySelectorAll(".copy-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var targetId = btn.getAttribute("data-copy-target");
      var codeEl = document.getElementById(targetId);
      if (!codeEl || !navigator.clipboard) return;

      navigator.clipboard.writeText(codeEl.innerText).then(function () {
        var original = btn.textContent;
        btn.textContent = "copied";
        btn.classList.add("copied");
        setTimeout(function () {
          btn.textContent = original;
          btn.classList.remove("copied");
        }, 1500);
      });
    });
  });

  /* ---------------------------------------------------------------
     Docs sidebar scroll-spy — highlights the section currently in
     view. Falls back to no highlighting (still fully navigable via
     the links themselves) if IntersectionObserver is unavailable.
     --------------------------------------------------------------- */
  var tocLinks = document.querySelectorAll("#docs-toc a");
  var docsSections = document.querySelectorAll(".docs-section[id]");

  if (tocLinks.length && docsSections.length && "IntersectionObserver" in window) {
    var linkById = {};
    tocLinks.forEach(function (link) {
      linkById[link.getAttribute("href").slice(1)] = link;
    });

    var tocObserver = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          var link = linkById[entry.target.id];
          if (!link) return;
          if (entry.isIntersecting) {
            tocLinks.forEach(function (l) {
              l.classList.remove("active");
            });
            link.classList.add("active");
          }
        });
      },
      { rootMargin: "-96px 0px -70% 0px", threshold: 0 }
    );

    docsSections.forEach(function (section) {
      tocObserver.observe(section);
    });
  }
})();
