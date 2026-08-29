import { copyFile, mkdir, readFile, stat } from "node:fs/promises";
import { dirname, resolve } from "node:path";

import { compareSemver, parseSemver } from "../npm/lib/semver.mjs";
import { manifestVersion } from "./lib/manifests.mjs";

function parseArguments(argv) {
  const options = {};
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    if (!["--source", "--destination", "--version"].includes(name) || !argv[index + 1]) {
      throw new Error(
        "usage: stage-package.mjs --source <file> --destination <file> --version <version>",
      );
    }
    options[name.slice(2)] = name === "--version" ? argv[index + 1] : resolve(argv[index + 1]);
    index += 1;
  }
  if (!options.source || !options.destination || !options.version) {
    throw new Error(
      "usage: stage-package.mjs --source <file> --destination <file> --version <version>",
    );
  }
  parseSemver(options.version);
  return options;
}

async function readIfPresent(path) {
  try {
    if ((await stat(path)).isFile()) return await readFile(path, "utf8");
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  return null;
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  const source = await readFile(options.source, "utf8");
  const sourceVersion = manifestVersion(options.source, source);
  if (sourceVersion !== options.version) {
    throw new Error(`source version ${sourceVersion} does not match ${options.version}`);
  }

  const destination = await readIfPresent(options.destination);
  if (destination === source) {
    console.log(`${options.destination} is already current`);
    return;
  }
  if (destination !== null) {
    const destinationVersion = manifestVersion(options.destination, destination);
    const comparison = compareSemver(destinationVersion, sourceVersion);
    if (comparison > 0) {
      throw new Error(
        `refusing to replace newer ${destinationVersion} with ${sourceVersion}`,
      );
    }
    if (comparison === 0) {
      throw new Error(
        `${options.destination} already has ${sourceVersion} with different content`,
      );
    }
  }

  await mkdir(dirname(options.destination), { recursive: true });
  await copyFile(options.source, options.destination);
  console.log(`Staged ${options.destination} at ${sourceVersion}`);
}

await main();
