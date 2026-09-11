"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  AUTO_REFRESH_MS,
  albumLabel,
  formatPathMap,
  normalizeEngine,
  parsePathMap,
  remediationHint,
} = require("./app.js");
const appSource = fs.readFileSync(path.join(__dirname, "app.js"), "utf8");

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

test("job render abandons stale responses after navigation", function () {
  const renderJob = appSource.slice(
    appSource.indexOf("async function renderJob"),
    appSource.indexOf("function attemptHistory"),
  );
  assert.match(
    renderJob,
    /await Promise\.all\([\s\S]*?if \(route\(\)\.name !== "job" \|\| route\(\)\.id !== id\) \{\s*return;\s*\}[\s\S]*?el\.view\.innerHTML =/,
  );
});

test("job UI uses import fields and action endpoint", function () {
  assert.match(appSource, /job\.import_status/);
  assert.match(appSource, /j\.import_error/);
  assert.match(appSource, /\/api\/v1\/jobs\/" \+ encodeURIComponent\(j\.id\) \+ "\/import"/);
});

test("settings UI uses Lidarr connector fields", function () {
  for (const key of [
    "lidarr_url",
    "lidarr_api_key",
    "lidarr_api_key_set",
    "lidarr_import_enabled",
    "lidarr_poll_interval_sec",
    "lidarr_path_map",
  ]) {
    assert.match(appSource, new RegExp(key));
  }
});

test("settings render abandons stale responses after navigation", function () {
  const renderSettings = appSource.slice(
    appSource.indexOf("async function renderSettings"),
    appSource.indexOf("function engineOpt"),
  );
  assert.match(
    renderSettings,
    /await api\([\s\S]*?if \(route\(\)\.name !== "settings"\) \{\s*return;\s*\}[\s\S]*?el\.view\.innerHTML =/,
  );
});
