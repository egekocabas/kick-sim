import { spawnSync } from "node:child_process";
import {
  chmod,
  copyFile,
  mkdir,
  readFile,
  rm,
  stat,
  writeFile,
} from "node:fs/promises";
import { basename, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { platforms } from "../lib/platforms.mjs";
import { isPrerelease, parseSemver } from "../lib/semver.mjs";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const packagingDirectory = resolve(scriptDirectory, "..");
const repositoryRoot = resolve(packagingDirectory, "../..");

function parseArguments(argv) {
  const options = {
    dist: resolve(repositoryRoot, "dist"),
    out: resolve(repositoryRoot, "dist/distribution/npm"),
  };
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    if (!["--dist", "--out", "--version"].includes(name)) {
      throw new Error(`unknown argument: ${name}`);
    }
    const value = argv[index + 1];
    if (!value) throw new Error(`missing value for ${name}`);
    options[name.slice(2)] = value;
    index += 1;
  }
  options.dist = resolve(options.dist);
  options.out = resolve(options.out);
  return options;
}

async function readJSON(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

async function resolveArtifactPath(artifact, distDirectory) {
  const candidates = [
    resolve(repositoryRoot, artifact.path),
    resolve(distDirectory, artifact.path),
    resolve(distDirectory, basename(artifact.path)),
  ];
  for (const candidate of candidates) {
    try {
      if ((await stat(candidate)).isFile()) return candidate;
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  throw new Error(`binary artifact not found: ${artifact.path}`);
}

function packageMetadata(name, version) {
  return {
    name,
    version,
    description:
      "Local-only simulator for building and testing Kick webhook integrations",
    license: "MIT",
    author: "Ege Kocabas",
    homepage: "https://github.com/egekocabas/kick-sim",
    repository: {
      type: "git",
      url: "git+https://github.com/egekocabas/kick-sim.git",
    },
    bugs: { url: "https://github.com/egekocabas/kick-sim/issues" },
    keywords: ["kick", "webhook", "simulator", "cli", "testing"],
    publishConfig: { access: "public" },
  };
}

async function writePackageFiles(directory) {
  await copyFile(resolve(repositoryRoot, "README.md"), resolve(directory, "README.md"));
  await copyFile(resolve(repositoryRoot, "LICENSE"), resolve(directory, "LICENSE"));
}

function packPackage(packageDirectory, tarballDirectory, cacheDirectory) {
  const result = spawnSync(
    "npm",
    [
      "pack",
      "--json",
      "--cache",
      cacheDirectory,
      "--pack-destination",
      tarballDirectory,
      packageDirectory,
    ],
    { cwd: repositoryRoot, encoding: "utf8" },
  );
  if (result.status !== 0) {
    throw new Error(`npm pack failed:\n${result.stderr || result.stdout}`);
  }
  const [packed] = JSON.parse(result.stdout);
  if (!packed?.filename || !packed.integrity) {
    throw new Error(`npm pack returned incomplete metadata for ${packageDirectory}`);
  }
  return packed;
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  const metadata = await readJSON(resolve(options.dist, "metadata.json"));
  const version = options.version ?? metadata.version;
  parseSemver(version);
  if (options.version && options.version !== metadata.version) {
    throw new Error(
      `requested npm version ${options.version} does not match GoReleaser ${metadata.version}`,
    );
  }

  const artifacts = await readJSON(resolve(options.dist, "artifacts.json"));
  const binaries = artifacts.filter(
    (artifact) => artifact.type === "Binary" && artifact.extra?.ID === "kick-sim",
  );

  await rm(options.out, { recursive: true, force: true });
  const packagesDirectory = resolve(options.out, "packages");
  const tarballsDirectory = resolve(options.out, "tarballs");
  const cacheDirectory = resolve(options.out, ".npm-cache");
  await mkdir(packagesDirectory, { recursive: true });
  await mkdir(tarballsDirectory, { recursive: true });

  const packedPackages = [];
  for (const platform of platforms) {
    const matches = binaries.filter(
      (artifact) =>
        artifact.goos === platform.goos && artifact.goarch === platform.goarch,
    );
    if (matches.length !== 1) {
      throw new Error(
        `expected one ${platform.goos}/${platform.goarch} binary, found ${matches.length}`,
      );
    }

    const packageDirectory = resolve(packagesDirectory, platform.packageName);
    const binDirectory = resolve(packageDirectory, "bin");
    await mkdir(binDirectory, { recursive: true });
    const source = await resolveArtifactPath(matches[0], options.dist);
    const destination = resolve(binDirectory, platform.executable);
    await copyFile(source, destination);
    await chmod(destination, 0o755);

    const manifest = {
      ...packageMetadata(platform.packageName, version),
      os: [platform.nodePlatform],
      cpu: [platform.nodeArch],
      files: ["bin"],
    };
    await writeFile(
      resolve(packageDirectory, "package.json"),
      `${JSON.stringify(manifest, null, 2)}\n`,
    );
    await writePackageFiles(packageDirectory);
    const packed = packPackage(packageDirectory, tarballsDirectory, cacheDirectory);
    const expectedBinary = `bin/${platform.executable}`;
    const binaryEntry = packed.files?.find((file) => file.path === expectedBinary);
    if (!binaryEntry) throw new Error(`${platform.packageName} omits ${expectedBinary}`);
    if (platform.goos !== "windows" && (binaryEntry.mode & 0o111) === 0) {
      throw new Error(`${platform.packageName} binary is not executable`);
    }
    packedPackages.push({
      name: platform.packageName,
      role: "platform",
      platform: platform.nodePlatform,
      arch: platform.nodeArch,
      executable: platform.executable,
      filename: packed.filename,
      integrity: packed.integrity,
      shasum: packed.shasum,
    });
  }

  const launcherDirectory = resolve(packagesDirectory, "kick-sim");
  await mkdir(resolve(launcherDirectory, "bin"), { recursive: true });
  await copyFile(
    resolve(packagingDirectory, "runtime/kick-sim.mjs"),
    resolve(launcherDirectory, "bin/kick-sim.mjs"),
  );
  await chmod(resolve(launcherDirectory, "bin/kick-sim.mjs"), 0o755);
  const launcherManifest = {
    ...packageMetadata("kick-sim", version),
    type: "module",
    engines: { node: ">=22.14.0" },
    bin: { "kick-sim": "bin/kick-sim.mjs" },
    files: ["bin"],
    optionalDependencies: Object.fromEntries(
      platforms.map((platform) => [platform.packageName, version]),
    ),
  };
  await writeFile(
    resolve(launcherDirectory, "package.json"),
    `${JSON.stringify(launcherManifest, null, 2)}\n`,
  );
  await writePackageFiles(launcherDirectory);
  const launcherPack = packPackage(
    launcherDirectory,
    tarballsDirectory,
    cacheDirectory,
  );
  if (launcherPack.files?.some((file) => file.path === "package.json" && file.size === 0)) {
    throw new Error("launcher package.json is empty");
  }
  packedPackages.push({
    name: "kick-sim",
    role: "launcher",
    filename: launcherPack.filename,
    integrity: launcherPack.integrity,
    shasum: launcherPack.shasum,
  });

  const releaseManifest = {
    version,
    tag: metadata.tag || `v${version}`,
    prerelease: isPrerelease(version),
    packages: packedPackages,
  };
  await writeFile(
    resolve(options.out, "release.json"),
    `${JSON.stringify(releaseManifest, null, 2)}\n`,
  );
  await rm(cacheDirectory, { recursive: true, force: true });
  await rm(packagesDirectory, { recursive: true, force: true });
  console.log(
    `Packed ${packedPackages.length} npm packages for ${version} in ${options.out}`,
  );
}

await main();
