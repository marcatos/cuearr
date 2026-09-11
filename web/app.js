(function () {
  "use strict";

  const API_KEY_STORAGE = "cuearr_api_key";
  const AUTO_REFRESH_MS = 5000;

  function apiKey() {
    return localStorage.getItem(API_KEY_STORAGE) || "";
  }

  async function api(path, options) {
    const opts = Object.assign({ credentials: "include" }, options || {});
    opts.headers = Object.assign({ Accept: "application/json" }, opts.headers || {});
    const key = apiKey();
    if (key) {
      opts.headers["X-Api-Key"] = key;
    }
    if (opts.body && typeof opts.body === "object" && !(opts.body instanceof FormData)) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(opts.body);
    }
    const res = await fetch(path, opts);
    let data = null;
    const ct = res.headers.get("Content-Type") || "";
    if (ct.includes("application/json")) {
      data = await res.json();
    }
    if (res.status === 401 && !path.includes("/login")) {
      location.hash = "#/login";
      throw new Error((data && data.error) || "unauthorized");
    }
    if (!res.ok) {
      throw new Error((data && data.error) || res.statusText || "request failed");
    }
    return data;
  }

  function albumLabel(cuePath) {
    const parts = String(cuePath || "")
      .replace(/\\/g, "/")
      .split("/")
      .filter(Boolean);
    return parts.length > 1 ? parts[parts.length - 2] : parts[0] || "Unknown album";
  }

  function remediationHint(error) {
    const message = String(error || "").toLowerCase();
    if (/permission|access denied|read-only/.test(message)) {
      return "Check read permissions on the source and write permissions on the output directory.";
    }
    if (/no space|disk full|insufficient.*space/.test(message)) {
      return "Free disk space in the output location, then retry the job.";
    }
    if (/multiple cue|ambiguous cue/.test(message)) {
      return "Keep one CUE sheet for this album, move the others elsewhere, then retry.";
    }
    if (/verif|mismatch|duration|track count/.test(message)) {
      return "Confirm the source image and CUE sheet belong to the same album and have matching track boundaries.";
    }
    return "Review the job log, correct the reported problem, then retry.";
  }

  function normalizeEngine(engine) {
    return engine === "native" ? "shntool" : engine || "shntool";
  }

  function formatPathMap(rules) {
    return (rules || [])
      .map(function (rule) {
        return String(rule.from || "") + "=>" + String(rule.to || "");
      })
      .join("\n");
  }

  function parsePathMap(text) {
    const rules = [];
    String(text || "")
      .split(/\r?\n/)
      .forEach(function (line, index) {
        const trimmed = line.trim();
        if (!trimmed) {
          return;
        }
        const separator = trimmed.indexOf("=>");
        const from = separator >= 0 ? trimmed.slice(0, separator).trim() : "";
        const to = separator >= 0 ? trimmed.slice(separator + 2).trim() : "";
        if (!from || !to) {
          throw new Error("Path map line " + (index + 1) + " must use from=>to.");
        }
        rules.push({ from: from, to: to });
      });
    return rules;
  }

  if (typeof module !== "undefined" && module.exports) {
    module.exports = {
      AUTO_REFRESH_MS: AUTO_REFRESH_MS,
      albumLabel: albumLabel,
      formatPathMap: formatPathMap,
      normalizeEngine: normalizeEngine,
      parsePathMap: parsePathMap,
      remediationHint: remediationHint,
    };
    return;
  }

  const el = {
    view: document.getElementById("view"),
    nav: document.getElementById("nav-main"),
    logout: document.getElementById("btn-logout"),
  };
  let refreshTimer = null;
  let refreshInFlight = false;

  function route() {
    const hash = location.hash.replace(/^#/, "") || "/";
    const parts = hash.split("/").filter(Boolean);
    if (parts[0] === "login") {
      return { name: "login" };
    }
    if (parts[0] === "jobs" && parts[1]) {
      return { name: "job", id: parts[1] };
    }
    if (parts[0] === "settings") {
      return { name: "settings" };
    }
    if (parts[0] === "history") {
      return { name: "history" };
    }
    return { name: "dashboard" };
  }

  function setNav(routeName) {
    const show = routeName !== "login";
    el.nav.classList.toggle("hidden", !show);
    el.nav.querySelectorAll("a[data-nav]").forEach(function (a) {
      a.classList.toggle("active", a.getAttribute("data-nav") === routeName);
    });
  }

  function esc(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function statusClass(st) {
    return "status status-" + (st || "queued");
  }

  function importStatus(job) {
    return job.import_status || "none";
  }

  async function loadJobs() {
    const data = await api("/api/v1/jobs?limit=500");
    return data.jobs || [];
  }

  function countByStatus(jobs) {
    const c = { queued: 0, running: 0, completed: 0, failed: 0 };
    jobs.forEach(function (j) {
      if (c[j.status] !== undefined) {
        c[j.status]++;
      }
    });
    return c;
  }

  function jobsTable(jobs, emptyMsg) {
    if (!jobs.length) {
      return "<p class=\"muted\">" + esc(emptyMsg) + "</p>";
    }
    let rows = jobs
      .map(function (j) {
        return (
          "<tr><td><a href=\"#/jobs/" +
          esc(j.id) +
          "\">" +
          "<strong>" +
          esc(albumLabel(j.cue_path)) +
          "</strong><span class=\"job-id\">" +
          esc(j.id.slice(0, 8)) +
          "</span>" +
          "</a></td><td><span class=\"" +
          statusClass(j.status) +
          "\">" +
          esc(j.status) +
          "</span></td><td>" +
          '<span class="' +
          statusClass(importStatus(j)) +
          '">' +
          esc(importStatus(j)) +
          "</span></td><td>" +
          esc(j.cue_path) +
          "</td><td>" +
          esc(j.engine) +
          "</td><td>" +
          esc(j.created_at) +
          "</td></tr>"
        );
      })
      .join("");
    return (
      "<table><thead><tr><th>Album</th><th>Split status</th><th>Import status</th><th>CUE</th><th>Engine</th><th>Created</th></tr></thead><tbody>" +
      rows +
      "</tbody></table>"
    );
  }

  async function renderDashboard(showLoading) {
    if (showLoading !== false) {
      el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    }
    const jobs = await loadJobs();
    if (route().name !== "dashboard") {
      return;
    }
    const all = jobs.filter(function (j) {
      return j.status === "queued" || j.status === "running";
    });
    const counts = countByStatus(jobs);
    el.view.innerHTML =
      "<h1>Queue</h1>" +
      '<div class="stats">' +
      stat("Queued", counts.queued) +
      stat("Running", counts.running) +
      stat("Completed", counts.completed) +
      stat("Failed", counts.failed) +
      "</div>" +
      '<div class="toolbar">' +
      '<button type="button" id="btn-scan">Scan watch dirs</button>' +
      '<button type="button" class="secondary" id="btn-refresh">Refresh</button>' +
      "</div>" +
      '<div class="card">' +
      jobsTable(all, "No jobs in queue.") +
      "</div>";
    document.getElementById("btn-refresh").onclick = function () {
      refreshCurrentView("dashboard");
    };
    document.getElementById("btn-scan").onclick = async function () {
      const btn = document.getElementById("btn-scan");
      btn.disabled = true;
      try {
        await api("/api/v1/jobs/scan", { method: "POST" });
        await renderDashboard(false);
      } catch (e) {
        alert(e.message);
      } finally {
        btn.disabled = false;
      }
    };
  }

  function stat(label, n) {
    return '<div class="stat"><div class="n">' + n + '</div><div class="l">' + esc(label) + "</div></div>";
  }

  async function renderHistory(showLoading) {
    if (showLoading !== false) {
      el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    }
    const jobs = await loadJobs();
    if (route().name !== "history") {
      return;
    }
    const hist = jobs.filter(function (j) {
      return j.status === "completed" || j.status === "failed";
    });
    el.view.innerHTML =
      "<h1>History</h1>" +
      '<div class="toolbar"><button type="button" class="secondary" id="btn-refresh">Refresh</button></div>' +
      '<div class="card">' +
      jobsTable(hist, "No completed or failed jobs yet.") +
      "</div>";
    document.getElementById("btn-refresh").onclick = function () {
      refreshCurrentView("history");
    };
  }

  async function renderJob(id) {
    el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    const responses = await Promise.all([
      api("/api/v1/jobs/" + encodeURIComponent(id)),
      api("/api/v1/settings"),
    ]);
    const j = responses[0];
    const settings = responses[1];
    if (route().name !== "job" || route().id !== id) {
      return;
    }
    const err = j.error ? '<p class="msg err">' + esc(j.error) + "</p>" : "";
    const importError = j.import_error
      ? '<p class="msg err"><strong>Import:</strong> ' + esc(j.import_error) + "</p>"
      : "";
    const importState = importStatus(j);
    const requestImportAction =
      j.status === "completed" && settings.lidarr_import_enabled
        ? '<button type="button" id="btn-import"' +
          (importState === "imported" || importState === "skipped"
            ? ' disabled title="This import cannot be requested again"'
            : "") +
          ">Request import</button>"
        : "";
    const failedActions =
      j.status === "failed"
        ? '<div class="remediation"><h2>How to recover</h2><p>' +
          esc(remediationHint(j.error)) +
          '</p><button type="button" id="btn-retry">Retry job</button></div>'
        : "";
    el.view.innerHTML =
      "<h1>Job " +
      esc(j.id) +
      "</h1>" +
      '<p><a href="#/">&larr; Back to queue</a></p>' +
      err +
      importError +
      '<div class="card">' +
      "<p><strong>Split status:</strong> <span class=\"" +
      statusClass(j.status) +
      "\">" +
      esc(j.status) +
      "</span> · <strong>Import status:</strong> <span class=\"" +
      statusClass(importState) +
      "\">" +
      esc(importState) +
      "</span> · " +
      esc(j.engine) +
      "</p>" +
      "<p><strong>CUE:</strong> " +
      esc(j.cue_path) +
      "</p>" +
      "<p><strong>Image:</strong> " +
      esc(j.image_path) +
      "</p>" +
      (j.out_dir ? "<p><strong>Out:</strong> " + esc(j.out_dir) + "</p>" : "") +
      "<p><strong>Attempts:</strong> " +
      esc(String(j.attempt_count || 0)) +
      "</p>" +
      (j.import_requested_at
        ? "<p><strong>Import requested:</strong> " + esc(j.import_requested_at) + "</p>"
        : "") +
      (j.import_finished_at
        ? "<p><strong>Import finished:</strong> " + esc(j.import_finished_at) + "</p>"
        : "") +
      requestImportAction +
      failedActions +
      attemptHistory(j.attempts) +
      "<h2>Log</h2>" +
      '<div class="log-box">' +
      esc(j.log || "(empty)") +
      "</div>" +
      "</div>";
    const retry = document.getElementById("btn-retry");
    if (retry) {
      retry.onclick = async function () {
        retry.disabled = true;
        try {
          await api("/api/v1/jobs/" + encodeURIComponent(j.id) + "/retry", { method: "POST" });
          await renderJob(j.id);
        } catch (e) {
          alert(e.message);
          retry.disabled = false;
        }
      };
    }
    const requestImport = document.getElementById("btn-import");
    if (requestImport) {
      requestImport.onclick = async function () {
        const startedAt = Date.now();
        requestImport.disabled = true;
        console.info("INFO import request started", { jobId: j.id });
        try {
          const updated = await api("/api/v1/jobs/" + encodeURIComponent(j.id) + "/import", {
            method: "POST",
          });
          console.info("INFO import request completed", {
            jobId: j.id,
            importStatus: updated.import_status,
            durationMs: Date.now() - startedAt,
          });
          await renderJob(j.id);
        } catch (e) {
          console.error("ERROR import request failed", {
            jobId: j.id,
            durationMs: Date.now() - startedAt,
          });
          alert(e.message);
          requestImport.disabled = false;
        }
      };
    }
  }

  function attemptHistory(attempts) {
    if (!attempts || !attempts.length) {
      return "";
    }
    const rows = attempts
      .map(function (a) {
        return (
          "<li>#" +
          esc(String(a.n)) +
          " · " +
          esc(a.at || "") +
          " · " +
          esc(a.error || "") +
          "</li>"
        );
      })
      .join("");
    return "<h2>Attempt history</h2><ul>" + rows + "</ul>";
  }

  async function renderSettings() {
    el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    const s = await api("/api/v1/settings");
    if (route().name !== "settings") {
      return;
    }
    const auth = s.auth || {};
    const watch = (s.watch_dirs || []).join("\n");
    const domains = (auth.oidc_allowed_email_domains || []).join(", ");
    const lidarrPathMap = formatPathMap(s.lidarr_path_map);
    const nativeWarning =
      s.engine === "native"
        ? '<p class="msg warn">The native engine is no longer supported. Save settings to switch to shntool.</p>'
        : "";
    el.view.innerHTML =
      "<h1>Settings</h1>" +
      '<form id="settings-form" class="card">' +
      nativeWarning +
      '<div class="form-row"><label>Watch directories (one per line)</label><textarea name="watch_dirs" rows="4">' +
      esc(watch) +
      "</textarea></div>" +
      '<div class="form-row"><label>Output directory</label><input type="text" name="out_dir" value="' +
      esc(s.out_dir || "") +
      '" /></div>' +
      '<div class="checkbox-row"><input type="checkbox" name="in_place" id="in_place"' +
      (s.in_place ? " checked" : "") +
      ' /><label for="in_place">In-place split</label></div>' +
      '<div class="form-row"><label>Engine</label><select name="engine">' +
      engineOpt("shntool", s.engine) +
      "</select></div>" +
      '<div class="form-row"><label>Max attempts (total, including first; default 3)</label><input type="number" min="1" name="max_retries" value="' +
      esc(String(s.max_retries != null ? s.max_retries : 3)) +
      '" /></div>' +
      "<h2>Lidarr connector</h2>" +
      '<div class="checkbox-row"><input type="checkbox" name="lidarr_import_enabled" id="lidarr_import_enabled"' +
      (s.lidarr_import_enabled ? " checked" : "") +
      ' /><label for="lidarr_import_enabled">Enable Lidarr import</label></div>' +
      '<div class="form-row"><label>Lidarr URL</label><input type="text" name="lidarr_url" value="' +
      esc(s.lidarr_url || "") +
      '" /></div>' +
      '<div class="form-row"><label>Lidarr API key (leave blank to keep)</label><input type="password" name="lidarr_api_key" autocomplete="new-password" placeholder="' +
      (s.lidarr_api_key_set ? "••••••••" : "") +
      '" /></div>' +
      '<div class="form-row"><label>Poll interval (seconds)</label><input type="number" min="1" name="lidarr_poll_interval_sec" value="' +
      esc(String(s.lidarr_poll_interval_sec != null ? s.lidarr_poll_interval_sec : 300)) +
      '" /></div>' +
      '<div class="form-row"><label>Path map (one from=&gt;to rule per line)</label><textarea name="lidarr_path_map" rows="4" placeholder="/downloads=&gt;/data">' +
      esc(lidarrPathMap) +
      "</textarea></div>" +
      "<h2>API access</h2>" +
      '<p class="muted">Optional API key sent as X-Api-Key (stored in this browser only).</p>' +
      '<div class="form-row"><label>Browser API key</label><input type="password" name="browser_api_key" autocomplete="off" value="' +
      esc(apiKey()) +
      '" /></div>' +
      "<h2>Auth</h2>" +
      '<div class="form-row"><label>New password (leave blank to keep)</label><input type="password" name="password" autocomplete="new-password" /></div>' +
      '<div class="form-row"><label>Server API key (leave blank to keep)</label><input type="password" name="api_key" autocomplete="off" placeholder="' +
      (auth.api_key_set ? "••••••••" : "") +
      '" /></div>' +
      '<p class="muted">Password configured: ' +
      (auth.password_configured ? "yes" : "no") +
      "</p>" +
      "<h2>OIDC</h2>" +
      '<div class="checkbox-row"><input type="checkbox" name="oidc_enabled" id="oidc_enabled"' +
      (auth.oidc_enabled ? " checked" : "") +
      ' /><label for="oidc_enabled">Enable OIDC login</label></div>' +
      '<div class="form-row"><label>Issuer URL</label><input type="text" name="oidc_issuer" value="' +
      esc(auth.oidc_issuer || "") +
      '" /></div>' +
      '<div class="form-row"><label>Client ID</label><input type="text" name="oidc_client_id" value="' +
      esc(auth.oidc_client_id || "") +
      '" /></div>' +
      '<div class="form-row"><label>Client secret (leave blank to keep)</label><input type="password" name="oidc_client_secret" autocomplete="off" placeholder="' +
      (auth.oidc_client_secret_set ? "••••••••" : "") +
      '" /></div>' +
      '<div class="form-row"><label>Redirect URL</label><input type="text" name="oidc_redirect_url" value="' +
      esc(auth.oidc_redirect_url || "") +
      '" /></div>' +
      '<div class="form-row"><label>Allowed email domains (comma-separated)</label><input type="text" name="oidc_domains" value="' +
      esc(domains) +
      '" /></div>' +
      '<p><a href="/api/v1/auth/oidc/login">Sign in with OIDC</a> (test provider redirect)</p>' +
      "<h2>Diagnostics</h2>" +
      '<p class="muted">Download a redacted snapshot for troubleshooting.</p>' +
      '<button type="button" class="secondary" id="btn-diagnostics">Download diagnostics</button>' +
      '<div id="settings-msg"></div>' +
      '<button type="submit">Save settings</button>' +
      "</form>";
    document.getElementById("btn-diagnostics").onclick = async function () {
      const btn = document.getElementById("btn-diagnostics");
      btn.disabled = true;
      try {
        const diagnostics = await api("/api/v1/diagnostics");
        const blob = new Blob([JSON.stringify(diagnostics, null, 2) + "\n"], { type: "application/json" });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.href = url;
        link.download = "cuearr-diagnostics.json";
        link.click();
        URL.revokeObjectURL(url);
      } catch (e) {
        alert(e.message);
      } finally {
        btn.disabled = false;
      }
    };
    document.getElementById("settings-form").onsubmit = async function (ev) {
      ev.preventDefault();
      const f = ev.target;
      const msg = document.getElementById("settings-msg");
      msg.innerHTML = "";
      const watchDirs = f.watch_dirs.value
        .split(/\r?\n/)
        .map(function (l) {
          return l.trim();
        })
        .filter(Boolean);
      const domainsRaw = f.oidc_domains.value.trim();
      const domainList = domainsRaw
        ? domainsRaw.split(",").map(function (d) {
            return d.trim();
          }).filter(Boolean)
        : [];
      const browserKey = f.browser_api_key.value.trim();
      if (browserKey) {
        localStorage.setItem(API_KEY_STORAGE, browserKey);
      } else {
        localStorage.removeItem(API_KEY_STORAGE);
      }
      let lidarrPathMap;
      try {
        lidarrPathMap = parsePathMap(f.lidarr_path_map.value);
      } catch (e) {
        msg.innerHTML = '<p class="msg err">' + esc(e.message) + "</p>";
        return;
      }
      const body = {
        watch_dirs: watchDirs,
        out_dir: f.out_dir.value.trim(),
        in_place: f.in_place.checked,
        engine: "shntool",
        max_retries: parseInt(f.max_retries.value, 10) || 3,
        lidarr_url: f.lidarr_url.value.trim(),
        lidarr_import_enabled: f.lidarr_import_enabled.checked,
        lidarr_poll_interval_sec: parseInt(f.lidarr_poll_interval_sec.value, 10) || 300,
        lidarr_path_map: lidarrPathMap,
        auth: {
          oidc_enabled: f.oidc_enabled.checked,
          oidc_issuer: f.oidc_issuer.value.trim(),
          oidc_client_id: f.oidc_client_id.value.trim(),
          oidc_redirect_url: f.oidc_redirect_url.value.trim(),
          oidc_allowed_email_domains: domainList,
        },
      };
      if (f.password.value) {
        body.auth.password = f.password.value;
      }
      if (f.api_key.value) {
        body.auth.api_key = f.api_key.value;
      }
      if (f.oidc_client_secret.value) {
        body.auth.oidc_client_secret = f.oidc_client_secret.value;
      }
      if (f.lidarr_api_key.value) {
        body.lidarr_api_key = f.lidarr_api_key.value;
      }
      try {
        await api("/api/v1/settings", { method: "PUT", body: body });
        msg.innerHTML = '<p class="msg ok">Saved.</p>';
        f.password.value = "";
        f.api_key.value = "";
        f.oidc_client_secret.value = "";
        f.lidarr_api_key.value = "";
        if (f.lidarr_api_key.placeholder || body.lidarr_api_key) {
          f.lidarr_api_key.placeholder = "••••••••";
        }
      } catch (e) {
        msg.innerHTML = '<p class="msg err">' + esc(e.message) + "</p>";
      }
    };
  }

  function engineOpt(val, current) {
    const sel = val === normalizeEngine(current) ? " selected" : "";
    return '<option value="' + val + '"' + sel + ">" + val + "</option>";
  }

  function stopAutoRefresh() {
    if (refreshTimer !== null) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
  }

  function startAutoRefresh(routeName) {
    stopAutoRefresh();
    if (document.visibilityState === "hidden") {
      return;
    }
    refreshTimer = setInterval(function () {
      refreshCurrentView(routeName);
    }, AUTO_REFRESH_MS);
  }

  async function refreshCurrentView(routeName) {
    if (refreshInFlight || document.visibilityState === "hidden" || route().name !== routeName) {
      return;
    }
    refreshInFlight = true;
    try {
      if (routeName === "dashboard") {
        await renderDashboard(false);
      } else if (routeName === "history") {
        await renderHistory(false);
      }
    } catch (e) {
      console.error("ERROR queue refresh failed", { route: routeName, error: e });
    } finally {
      refreshInFlight = false;
    }
  }

  function renderLogin() {
    el.view.innerHTML =
      '<div class="login-wrap card">' +
      "<h1>Log in</h1>" +
      '<form id="login-form">' +
      '<div class="form-row"><label>Password</label><input type="password" name="password" autocomplete="current-password" required /></div>' +
      '<div id="login-msg"></div>' +
      '<button type="submit">Sign in</button>' +
      "</form>" +
      '<p class="muted" style="margin-top:1rem"><a href="/api/v1/auth/oidc/login">OIDC login</a></p>' +
      "</div>";
    document.getElementById("login-form").onsubmit = async function (ev) {
      ev.preventDefault();
      const msg = document.getElementById("login-msg");
      msg.innerHTML = "";
      const password = ev.target.password.value;
      try {
        await api("/api/v1/login", { method: "POST", body: { password: password } });
        location.hash = "#/";
      } catch (e) {
        msg.innerHTML = '<p class="msg err">' + esc(e.message) + "</p>";
      }
    };
  }

  async function render() {
    stopAutoRefresh();
    const r = route();
    setNav(r.name === "job" ? "dashboard" : r.name);
    try {
      if (r.name === "login") {
        renderLogin();
        return;
      }
      if (r.name === "dashboard") {
        await renderDashboard();
        if (route().name === "dashboard") {
          startAutoRefresh("dashboard");
        }
        return;
      }
      if (r.name === "history") {
        await renderHistory();
        if (route().name === "history") {
          startAutoRefresh("history");
        }
        return;
      }
      if (r.name === "settings") {
        await renderSettings();
        return;
      }
      if (r.name === "job") {
        await renderJob(r.id);
        return;
      }
    } catch (e) {
      if (r.name !== "login") {
        el.view.innerHTML = '<p class="msg err">' + esc(e.message) + "</p>";
      }
    }
  }

  el.logout.onclick = async function () {
    try {
      await api("/api/v1/logout", { method: "POST" });
    } catch (_) {
      /* still leave session UI even if network fails */
    }
    window.location.href = "/login";
  };

  window.addEventListener("hashchange", function () {
    stopAutoRefresh();
    render();
  });
  document.addEventListener("visibilitychange", function () {
    const current = route().name;
    if (document.visibilityState === "hidden") {
      stopAutoRefresh();
    } else if (current === "dashboard" || current === "history") {
      refreshCurrentView(current);
      startAutoRefresh(current);
    }
  });
  if (!location.hash) {
    location.hash = "#/";
  }
  render();
})();
