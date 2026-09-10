(function () {
  "use strict";

  const API_KEY_STORAGE = "cuearr_api_key";

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

  const el = {
    view: document.getElementById("view"),
    nav: document.getElementById("nav-main"),
    logout: document.getElementById("btn-logout"),
  };

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
          esc(j.id.slice(0, 8)) +
          "</a></td><td><span class=\"" +
          statusClass(j.status) +
          "\">" +
          esc(j.status) +
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
      "<table><thead><tr><th>ID</th><th>Status</th><th>CUE</th><th>Engine</th><th>Created</th></tr></thead><tbody>" +
      rows +
      "</tbody></table>"
    );
  }

  async function renderDashboard() {
    el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    const jobs = await loadJobs();
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
      renderDashboard();
    };
    document.getElementById("btn-scan").onclick = async function () {
      const btn = document.getElementById("btn-scan");
      btn.disabled = true;
      try {
        await api("/api/v1/jobs/scan", { method: "POST" });
        await renderDashboard();
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

  async function renderHistory() {
    el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    const jobs = await loadJobs();
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
      renderHistory();
    };
  }

  async function renderJob(id) {
    el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    const j = await api("/api/v1/jobs/" + encodeURIComponent(id));
    const err = j.error ? '<p class="msg err">' + esc(j.error) + "</p>" : "";
    el.view.innerHTML =
      "<h1>Job " +
      esc(j.id) +
      "</h1>" +
      '<p><a href="#/">&larr; Back to queue</a></p>' +
      err +
      '<div class="card">' +
      "<p><span class=\"" +
      statusClass(j.status) +
      "\">" +
      esc(j.status) +
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
      "<h2>Log</h2>" +
      '<div class="log-box">' +
      esc(j.log || "(empty)") +
      "</div>" +
      "</div>";
  }

  async function renderSettings() {
    el.view.innerHTML = "<p class=\"muted\">Loading…</p>";
    const s = await api("/api/v1/settings");
    const auth = s.auth || {};
    const watch = (s.watch_dirs || []).join("\n");
    const domains = (auth.oidc_allowed_email_domains || []).join(", ");
    el.view.innerHTML =
      "<h1>Settings</h1>" +
      '<form id="settings-form" class="card">' +
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
      engineOpt("native", s.engine) +
      "</select></div>" +
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
      '<div id="settings-msg"></div>' +
      '<button type="submit">Save settings</button>' +
      "</form>";
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
      const body = {
        watch_dirs: watchDirs,
        out_dir: f.out_dir.value.trim(),
        in_place: f.in_place.checked,
        engine: f.engine.value,
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
      try {
        await api("/api/v1/settings", { method: "PUT", body: body });
        msg.innerHTML = '<p class="msg ok">Saved.</p>';
        f.password.value = "";
        f.api_key.value = "";
        f.oidc_client_secret.value = "";
      } catch (e) {
        msg.innerHTML = '<p class="msg err">' + esc(e.message) + "</p>";
      }
    };
  }

  function engineOpt(val, current) {
    const sel = val === (current || "shntool") ? " selected" : "";
    return '<option value="' + val + '"' + sel + ">" + val + "</option>";
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
    const r = route();
    setNav(r.name === "job" ? "dashboard" : r.name);
    try {
      if (r.name === "login") {
        renderLogin();
        return;
      }
      if (r.name === "dashboard") {
        await renderDashboard();
        return;
      }
      if (r.name === "history") {
        await renderHistory();
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

  window.addEventListener("hashchange", render);
  if (!location.hash) {
    location.hash = "#/";
  }
  render();
})();
