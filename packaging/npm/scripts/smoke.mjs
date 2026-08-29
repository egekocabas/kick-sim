import { spawn, spawnSync } from "node:child_process";
import { chmod, cp, mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { delimiter, resolve } from "node:path";

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: "utf8", ...options });
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed:\n${result.stdout}\n${result.stderr}`,
    );
  }
  return result.stdout;
}

async function waitForStudio(url) {
  let lastError;
  for (let attempt = 0; attempt < 40; attempt += 1) {
    try {
      const response = await fetch(url);
      if (response.ok) return await response.text();
      lastError = new Error(`Studio returned HTTP ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await new Promise((accept) => setTimeout(accept, 250));
  }
  throw lastError ?? new Error("Studio did not start");
}

async function extractPackage(tarball, destination, fixture) {
  const extracted = await mkdtemp(resolve(fixture, "extract-"));
  run("tar", ["-xzf", tarball, "-C", extracted]);
  await cp(resolve(extracted, "package"), destination, { recursive: true });
}

async function main() {
  const bundleIndex = process.argv.indexOf("--bundle");
  const versionIndex = process.argv.indexOf("--version");
  const bundle = bundleIndex >= 0 ? resolve(process.argv[bundleIndex + 1] ?? "") : null;
  const requestedVersion = versionIndex >= 0 ? process.argv[versionIndex + 1] : null;
  if ((!bundle && !requestedVersion) || (bundle && requestedVersion)) {
    throw new Error(
      "usage: smoke.mjs (--bundle <npm bundle directory> | --version <published version>)",
    );
  }

  const fixture = await mkdtemp(resolve(tmpdir(), "kick-sim-npm-smoke-"));
  try {
    let expectedVersion;
    if (bundle) {
      const release = JSON.parse(
        await readFile(resolve(bundle, "release.json"), "utf8"),
      );
      const platform = release.packages.find(
        (entry) =>
          entry.role === "platform" &&
          entry.platform === process.platform &&
          entry.arch === process.arch,
      );
      const launcher = release.packages.find((entry) => entry.role === "launcher");
      if (!platform || !launcher) {
        throw new Error(`no smoke package for ${process.platform}/${process.arch}`);
      }
      const modules = resolve(fixture, "node_modules");
      await mkdir(modules, { recursive: true });
      await extractPackage(
        resolve(bundle, "tarballs", platform.filename),
        resolve(modules, platform.name),
        fixture,
      );
      await extractPackage(
        resolve(bundle, "tarballs", launcher.filename),
        resolve(modules, launcher.name),
        fixture,
      );
      await chmod(resolve(modules, platform.name, "bin", platform.executable), 0o755);
      expectedVersion = release.version;
    } else {
      const npmEnvironment = {
        ...process.env,
        npm_config_cache: resolve(fixture, ".npm-cache"),
      };
      run("npm", ["init", "--yes"], { cwd: fixture, env: npmEnvironment });
      run("npm", ["install", "--ignore-scripts", `kick-sim@${requestedVersion}`], {
        cwd: fixture,
        env: npmEnvironment,
      });
      expectedVersion = requestedVersion;
    }
    const launcherPath = resolve(fixture, "node_modules/kick-sim/bin/kick-sim.mjs");
    const workspace = resolve(fixture, "workspace");
    const environment = {
      ...process.env,
      PATH: `${resolve(fixture, "node_modules/.bin")}${delimiter}${process.env.PATH}`,
    };

    run(process.execPath, [launcherPath, "--help"], { cwd: fixture, env: environment });
    const versionOutput = run(
      process.execPath,
      [launcherPath, "--output", "json", "version"],
      { cwd: fixture, env: environment },
    );
    const version = JSON.parse(versionOutput);
    if (version.version !== expectedVersion) {
      throw new Error(`expected version ${expectedVersion}, received ${version.version}`);
    }
    run(process.execPath, [launcherPath, "--workspace", workspace, "init"], {
      cwd: fixture,
      env: environment,
    });
    run(
      process.execPath,
      [launcherPath, "--workspace", workspace, "workspace", "validate"],
      { cwd: fixture, env: environment },
    );

    const address = "127.0.0.1:14321";
    const studio = spawn(
      process.execPath,
      [
        launcherPath,
        "--workspace",
        workspace,
        "studio",
        "--no-open",
        "--address",
        address,
      ],
      { cwd: fixture, env: environment, stdio: ["ignore", "pipe", "pipe"] },
    );
    try {
      const html = await waitForStudio(`http://${address}/`);
      if (!html.includes("Kick Sim") || html.includes("assets are not embedded")) {
        throw new Error("npm binary did not serve the embedded Studio bundle");
      }
    } finally {
      studio.kill("SIGTERM");
      await Promise.race([
        new Promise((accept) => studio.once("exit", accept)),
        new Promise((accept) => setTimeout(accept, 5_000)),
      ]);
      if (studio.exitCode === null && studio.signalCode === null) studio.kill("SIGKILL");
    }
  } finally {
    await rm(fixture, { recursive: true, force: true });
  }
  console.log(`npm package smoke test passed on ${process.platform}/${process.arch}`);
}

await main();
