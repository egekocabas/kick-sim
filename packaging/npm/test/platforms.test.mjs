import assert from "node:assert/strict";
import test from "node:test";

import { platformForGo, platformForNode, platforms } from "../lib/platforms.mjs";
import {
  compareSemver,
  isPrerelease,
  parseSemver,
} from "../lib/semver.mjs";
import { packageForPlatform } from "../runtime/kick-sim.mjs";

test("all GoReleaser targets map to npm runtime packages", () => {
  assert.equal(platforms.length, 5);
  for (const platform of platforms) {
    assert.deepEqual(
      platformForGo(platform.goos, platform.goarch),
      platform,
    );
    assert.deepEqual(
      platformForNode(platform.nodePlatform, platform.nodeArch),
      platform,
    );
    assert.deepEqual(
      packageForPlatform(platform.nodePlatform, platform.nodeArch),
      [platform.packageName, platform.executable],
    );
  }
  assert.equal(packageForPlatform("freebsd", "x64"), undefined);
});

test("semantic versions compare without allowing channel downgrades", () => {
  assert.equal(compareSemver("0.5.0", "0.5.0"), 0);
  assert.equal(compareSemver("0.5.0-rc.2", "0.5.0-rc.10"), -1);
  assert.equal(compareSemver("0.5.0-rc.1", "0.5.0"), -1);
  assert.equal(compareSemver("0.6.0", "0.5.9"), 1);
  assert.equal(isPrerelease("0.5.0-rc.1"), true);
  assert.equal(isPrerelease("0.5.0"), false);
  assert.throws(() => parseSemver("v0.5.0"), /invalid semantic version/);
});
