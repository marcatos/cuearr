"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { AUTO_REFRESH_MS, albumLabel, normalizeEngine, remediationHint } = require("./app.js");

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
