import { spawnSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

import { compareSemver, isPrerelease, parseSemver } from "../lib/semver.mjs";

function runNpm(args, { allowMissing = false } = {}) {
  const result = spawnSync("npm", args, { encoding: "utf8" });
  if (result.status === 0) return result.stdout.trim();
  const output = `${result.stdout}\n${result.stderr}`;
  if (allowMissing && /E404|is not in this registry|No match found/i.test(output)) {
    return null;
  }
  throw new Error(`npm ${args.join(" ")} failed:\n${output.trim()}`);
}

function readJSONOutput(output, fallback) {
  if (!output) return fallback;
  return JSON.parse(output);
}

function remoteIntegrity(name, version) {
  const output = runNpm(
    ["view", `${name}@${version}`, "dist.integrity", "--json"],
    { allowMissing: true },
  );
  return readJSONOutput(output, null);
}

function distTags(name) {
  return readJSONOutput(runNpm(["view", name, "dist-tags", "--json"], { allowMissing: true }), {});
}

function assertTagDoesNotDowngrade(name, version, tag) {
  const current = distTags(name)[tag];
  if (current && compareSemver(current, version) > 0) {
    throw new Error(
      `refusing to move ${name}@${tag} backwards from ${current} to ${version}`,
    );
  }
}

function ensureDistTag(name, version, tag) {
  assertTagDoesNotDowngrade(name, version, tag);
  const current = distTags(name)[tag];
  if (current !== version) {
    throw new Error(
      `${name}@${version} exists, but ${tag} points to ${current ?? "nothing"}; ` +
        "repair the dist-tag interactively before rerunning",
    );
  }
}

async function waitForPackage(packageEntry, version) {
  for (let attempt = 1; attempt <= 60; attempt += 1) {
    const integrity = remoteIntegrity(packageEntry.name, version);
    if (integrity === packageEntry.integrity) return;
    if (integrity && integrity !== packageEntry.integrity) {
      throw new Error(
        `${packageEntry.name}@${version} exists with unexpected integrity ${integrity}`,
      );
    }
    if (attempt < 60) await new Promise((accept) => setTimeout(accept, 10_000));
  }
  throw new Error(`${packageEntry.name}@${version} did not become available within 10 minutes`);
}

async function publishPackage(packageEntry, version, tag, bundleDirectory) {
  const integrity = remoteIntegrity(packageEntry.name, version);
  if (integrity && integrity !== packageEntry.integrity) {
    throw new Error(
      `${packageEntry.name}@${version} already exists with different integrity ${integrity}`,
    );
  }
  assertTagDoesNotDowngrade(packageEntry.name, version, tag);
  if (!integrity) {
    const tarball = resolve(bundleDirectory, "tarballs", packageEntry.filename);
    runNpm(["publish", tarball, "--access", "public", "--tag", tag]);
  }
  await waitForPackage(packageEntry, version);
  ensureDistTag(packageEntry.name, version, tag);
  console.log(`${packageEntry.name}@${version} is available on ${tag}`);
}

async function main() {
  const bundleIndex = process.argv.indexOf("--bundle");
  if (bundleIndex < 0 || !process.argv[bundleIndex + 1]) {
    throw new Error("usage: publish.mjs --bundle <npm bundle directory>");
  }
  const bundleDirectory = resolve(process.argv[bundleIndex + 1]);
  const manifest = JSON.parse(
    await readFile(resolve(bundleDirectory, "release.json"), "utf8"),
  );
  parseSemver(manifest.version);
  if (manifest.prerelease !== isPrerelease(manifest.version)) {
    throw new Error("release manifest prerelease flag does not match its version");
  }
  const tag = manifest.prerelease ? "next" : "latest";
  const platformPackages = manifest.packages.filter(
    (packageEntry) => packageEntry.role === "platform",
  );
  const launchers = manifest.packages.filter(
    (packageEntry) => packageEntry.role === "launcher",
  );
  if (platformPackages.length !== 5 || launchers.length !== 1) {
    throw new Error("release manifest must contain five platform packages and one launcher");
  }

  for (const packageEntry of platformPackages) {
    await publishPackage(packageEntry, manifest.version, tag, bundleDirectory);
  }
  await publishPackage(launchers[0], manifest.version, tag, bundleDirectory);
}

await main();
