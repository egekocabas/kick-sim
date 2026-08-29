import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import test from "node:test";

const script = resolve(import.meta.dirname, "stage-package.mjs");

function cask(version, marker = "") {
  return `cask "kick-sim" do\n  version "${version}"\n  ${marker}\nend\n`;
}

function stage(source, destination, version) {
  return spawnSync(
    process.execPath,
    [script, "--source", source, "--destination", destination, "--version", version],
    { encoding: "utf8" },
  );
}

test("stages upgrades and rejects conflicting or older manifests", async () => {
  const directory = await mkdtemp(resolve(tmpdir(), "kick-sim-stage-test-"));
  const source = resolve(directory, "source.rb");
  const destination = resolve(directory, "destination.rb");
  try {
    await writeFile(source, cask("0.6.0", "sha256 \"new\""));
    await writeFile(destination, cask("0.5.0", "sha256 \"old\""));
    assert.equal(stage(source, destination, "0.6.0").status, 0);
    assert.equal(await readFile(destination, "utf8"), await readFile(source, "utf8"));

    assert.equal(stage(source, destination, "0.6.0").status, 0);
    await writeFile(source, cask("0.6.0", "sha256 \"different\""));
    assert.notEqual(stage(source, destination, "0.6.0").status, 0);

    await writeFile(source, cask("0.5.1", "sha256 \"older\""));
    assert.notEqual(stage(source, destination, "0.5.1").status, 0);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
