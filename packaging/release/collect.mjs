import { copyFile, mkdir, readFile, readdir, stat } from "node:fs/promises";
import { basename, resolve } from "node:path";

import { manifestVersion } from "./lib/manifests.mjs";

function parseArguments(argv) {
  const options = {};
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    if (!["--dist", "--out"].includes(name) || !argv[index + 1]) {
      throw new Error("usage: collect.mjs --dist <GoReleaser dist> --out <bundle>");
    }
    options[name.slice(2)] = resolve(argv[index + 1]);
    index += 1;
  }
  if (!options.dist || !options.out) {
    throw new Error("usage: collect.mjs --dist <GoReleaser dist> --out <bundle>");
  }
  return options;
}

async function walk(directory, excludedDirectory) {
  const paths = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (path === excludedDirectory) continue;
    if (entry.isDirectory()) paths.push(...(await walk(path, excludedDirectory)));
    else if (entry.isFile()) paths.push(path);
  }
  return paths;
}

async function findOne(paths, filename) {
  const matches = paths.filter((path) => basename(path) === filename);
  if (matches.length !== 1) {
    throw new Error(`expected one generated ${filename}, found ${matches.length}`);
  }
  return matches[0];
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  if (!(await stat(options.dist)).isDirectory()) {
    throw new Error(`GoReleaser dist is not a directory: ${options.dist}`);
  }
  const metadata = JSON.parse(
    await readFile(resolve(options.dist, "metadata.json"), "utf8"),
  );
  const paths = await walk(options.dist, options.out);
  const caskSource = await findOne(paths, "kick-sim.rb");
  const scoopSource = await findOne(paths, "kick-sim.json");
  const cask = await readFile(caskSource, "utf8");
  const scoop = await readFile(scoopSource, "utf8");
  for (const [path, contents] of [
    [caskSource, cask],
    [scoopSource, scoop],
  ]) {
    const version = manifestVersion(path, contents);
    if (version !== metadata.version) {
      throw new Error(`${basename(path)} has version ${version}, expected ${metadata.version}`);
    }
  }

  await mkdir(resolve(options.out, "homebrew"), { recursive: true });
  await mkdir(resolve(options.out, "scoop"), { recursive: true });
  await copyFile(caskSource, resolve(options.out, "homebrew/kick-sim.rb"));
  await copyFile(scoopSource, resolve(options.out, "scoop/kick-sim.json"));
  await copyFile(resolve(options.dist, "metadata.json"), resolve(options.out, "metadata.json"));
  console.log(`Collected Homebrew and Scoop manifests for ${metadata.version}`);
}

await main();
