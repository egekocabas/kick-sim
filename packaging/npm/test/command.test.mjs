import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import test from "node:test";

import { run, runNpm } from "../lib/command.mjs";

test("command startup failures include the underlying error", () => {
  assert.throws(() => run("kick-sim-command-that-does-not-exist", []), (error) => {
    assert.equal(error.cause.code, "ENOENT");
    assert.match(error.message, /Unable to start.*ENOENT/);
    return true;
  });
});

test("command failures retain exit status and stderr", () => {
  assert.throws(
    () => run(process.execPath, ["-e", 'console.error("test failure"); process.exit(7)']),
    /exit 7[\s\S]*test failure/,
  );
});

test("npm starts in a working directory containing spaces", async () => {
  const directory = await mkdtemp(resolve(tmpdir(), "kick-sim npm command "));
  try {
    const output = runNpm(["--version"], { cwd: directory });
    assert.match(output.trim(), /^\d+\.\d+\.\d+$/);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
