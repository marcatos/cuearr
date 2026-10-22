"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const {
  AUTO_REFRESH_MS,
  albumLabel,
  formatPathMap,
  normalizeEngine,
  parsePathMap,
  parsePollInterval,
  remediationHint,
} = require("./app.js");
const appSource = fs.readFileSync(path.join(__dirname, "app.js"), "utf8");

function browserAt(hash) {
  const requests = [];
  const listeners = {};
  const elements = new Map();
  const storage = new Map();
  function element(id) {
    if (!elements.has(id)) {
      elements.set(id, {
        innerHTML: "",
        classList: { toggle() {} },
        querySelectorAll() { return []; },
      });
    }
    return elements.get(id);
  }
  const location = { hash };
  vm.runInNewContext(appSource, {
    location,
    document: {
      getElementById: element,
      visibilityState: "visible",
      addEventListener() {},
    },
    window: { location, addEventListener(name, callback) { listeners[name] = callback; } },
    localStorage: {
      getItem(key) { return storage.get(key); },
      setItem(key, value) { storage.set(key, value); },
      removeItem(key) { storage.delete(key); },
    },
    FormData,
    setInterval() { return 1; },
    clearInterval() {},
    console: { info() {}, error() {} },
    alert(message) { throw new Error(message); },
    fetch(url, options) {
      return new Promise((resolve) => requests.push({ url, options, resolve }));
    },
  });
  return {
    requests,
    element,
    navigate(hash) { location.hash = hash; listeners.hashchange(); },
    respond(index, data) {
      assert.ok(requests[index], "missing request " + index);
      requests[index].resolve({
        ok: true,
        status: 200,
        headers: { get() { return "application/json"; } },
        async json() { return data; },
      });
    },
  };
}

function settle() { return new Promise(setImmediate); }

test("poll interval preserves disabled polling and rejects invalid values", function () {
  assert.equal(parsePollInterval("0"), 0);
  assert.equal(parsePollInterval("30"), 30);
  assert.equal(parsePollInterval(""), 300);
  for (const value of ["-1", "1.5", "invalid", "9007199254740992"]) {
    assert.throws(() => parsePollInterval(value), /whole number/);
  }
});

test("albumLabel returns the CUE parent directory on either path style", function () {
  assert.equal(albumLabel("/music/Massive Attack/Mezzanine/disc.cue"), "Mezzanine");
  assert.equal(albumLabel("D:\\Music\\Portishead\\Dummy\\album.cue"), "Dummy");
});

test("remediationHint maps common operational failures", function () {
  assert.match(remediationHint("permission denied writing tracks"), /permissions/i);
  assert.match(remediationHint("no space left on device"), /disk space/i);
  assert.match(remediationHint("insufficient free space"), /disk space/i);
  assert.match(remediationHint("multiple CUE sheets found"), /one CUE/i);
  assert.match(remediationHint("verification duration mismatch"), /source image and CUE/i);
});

test("remediationHint gives failed jobs a useful fallback", function () {
  assert.match(remediationHint("unexpected splitter exit"), /job log/i);
});

test("operational settings normalize legacy native engines", function () {
  assert.equal(normalizeEngine("native"), "shntool");
  assert.equal(normalizeEngine("shntool"), "shntool");
  assert.equal(AUTO_REFRESH_MS, 5000);
});

test("Lidarr path maps round-trip through from=>to text", function () {
  const rules = [
    { from: "C:\\downloads", to: "/downloads" },
    { from: "/music/source", to: "/music/import" },
  ];
  assert.deepEqual(parsePathMap(formatPathMap(rules)), rules);
  assert.deepEqual(parsePathMap("\n /from => /to \n"), [{ from: "/from", to: "/to" }]);
});

test("Lidarr path map rejects malformed rules", function () {
  assert.throws(() => parsePathMap("/source-only"), /line 1.*from=>to/i);
  assert.throws(() => parsePathMap("=> /target"), /line 1.*from=>to/i);
});

test("late job responses do not replace the login page", async function () {
  const browser = browserAt("#/jobs/old-job");
  browser.navigate("#/login");
  const login = browser.element("view").innerHTML;
  browser.respond(0, { id: "old-job" });
  browser.respond(1, {});
  await settle();
  assert.equal(browser.element("view").innerHTML, login);
});

test("late responses for a different job do not replace the current job", async function () {
  const browser = browserAt("#/jobs/old-job");
  browser.navigate("#/jobs/new-job");
  browser.respond(2, { id: "new-job", status: "running" });
  browser.respond(3, {});
  await settle();
  const current = browser.element("view").innerHTML;
  assert.match(current, /Job new-job/);
  browser.respond(0, { id: "old-job", status: "failed" });
  browser.respond(1, {});
  await settle();
  assert.equal(browser.element("view").innerHTML, current);
});

test("import button posts the request and renders its result", async function () {
  const browser = browserAt("#/jobs/job-1");
  const job = { id: "job-1", status: "completed", import_status: "failed", import_error: "Lidarr unavailable" };
  browser.respond(0, job);
  browser.respond(1, { lidarr_import_enabled: true });
  await settle();
  assert.match(browser.element("view").innerHTML, /Lidarr unavailable/);
  assert.match(browser.element("view").innerHTML, /Request import/);
  const click = browser.element("btn-import").onclick();
  assert.equal(browser.requests[2].url, "/api/v1/jobs/job-1/import");
  assert.equal(browser.requests[2].options.method, "POST");
  browser.respond(2, { import_status: "imported" });
  await settle();
  browser.respond(3, { ...job, import_status: "imported", import_error: "" });
  browser.respond(4, { lidarr_import_enabled: true });
  await click;
  assert.match(browser.element("view").innerHTML, /status-imported/);
  assert.doesNotMatch(browser.element("view").innerHTML, /Lidarr unavailable/);
});

test("late settings responses do not replace another page", async function () {
  const browser = browserAt("#/settings");
  browser.navigate("#/login");
  const login = browser.element("view").innerHTML;
  browser.respond(0, { engine: "shntool" });
  await settle();
  assert.equal(browser.element("view").innerHTML, login);
});

test("settings submission preserves zero polling and sends connector values", async function () {
  const browser = browserAt("#/settings");
  browser.respond(0, { engine: "shntool", lidarr_poll_interval_sec: 0 });
  await settle();
  assert.match(browser.element("view").innerHTML, /min="0" name="lidarr_poll_interval_sec" value="0"/);
  const form = {};
  for (const name of ["watch_dirs", "out_dir", "in_place", "max_retries", "lidarr_url",
    "lidarr_import_enabled", "lidarr_poll_interval_sec", "lidarr_path_map", "oidc_domains",
    "browser_api_key", "oidc_enabled", "oidc_issuer", "oidc_client_id", "oidc_redirect_url",
    "password", "api_key", "oidc_client_secret", "lidarr_api_key"]) {
    form[name] = { value: "", checked: false };
  }
  form.watch_dirs.value = "/watch\n /other ";
  form.out_dir.value = "/out";
  form.lidarr_url.value = "http://lidarr:8686";
  form.lidarr_import_enabled.checked = true;
  form.lidarr_poll_interval_sec.value = "0";
  form.lidarr_path_map.value = "/out=>/imports";
  const submit = browser.element("settings-form").onsubmit({ preventDefault() {}, target: form });
  assert.equal(browser.requests[1].url, "/api/v1/settings");
  assert.equal(browser.requests[1].options.method, "PUT");
  const body = JSON.parse(browser.requests[1].options.body);
  assert.equal(body.lidarr_poll_interval_sec, 0);
  assert.equal(body.lidarr_import_enabled, true);
  assert.equal(body.lidarr_url, "http://lidarr:8686");
  assert.deepEqual(body.watch_dirs, ["/watch", "/other"]);
  assert.deepEqual(body.lidarr_path_map, [{ from: "/out", to: "/imports" }]);
  assert.equal(Object.hasOwn(body, "lidarr_api_key"), false);
  browser.respond(1, {});
  await submit;
  assert.match(browser.element("settings-msg").innerHTML, /Saved/);
});

test("publishing jobs remain visible in the active queue", async function () {
  const browser = browserAt("#/");
  browser.respond(0, { jobs: [{ id: "publishing-job", status: "publishing", cue_path: "/album/album.cue", engine: "shntool" }] });
  await settle();
  assert.match(browser.element("view").innerHTML, /publishing-job/);
  assert.match(browser.element("view").innerHTML, /status-publishing/);
});
