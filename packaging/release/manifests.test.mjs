import assert from "node:assert/strict";
import test from "node:test";

import { manifestVersion } from "./lib/manifests.mjs";

test("extracts generated package-manager versions", () => {
  assert.equal(
    manifestVersion("kick-sim.rb", 'cask "kick-sim" do\n  version "0.5.0-rc.1"\nend\n'),
    "0.5.0-rc.1",
  );
  assert.equal(
    manifestVersion("kick-sim.json", '{"version":"0.5.0"}'),
    "0.5.0",
  );
});

test("rejects missing or invalid versions", () => {
  assert.throws(() => manifestVersion("kick-sim.rb", "cask do\nend\n"), /no version/);
  assert.throws(
    () => manifestVersion("kick-sim.json", '{"version":"next"}'),
    /invalid semantic version/,
  );
});
