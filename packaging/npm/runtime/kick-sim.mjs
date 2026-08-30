#!/usr/bin/env node

import { spawn } from "node:child_process";
import { realpathSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);

const packages = Object.freeze({
  "darwin-x64": ["kick-sim-darwin-x64", "kick-sim"],
  "darwin-arm64": ["kick-sim-darwin-arm64", "kick-sim"],
  "linux-x64": ["kick-sim-linux-x64", "kick-sim"],
  "linux-arm64": ["kick-sim-linux-arm64", "kick-sim"],
  "win32-x64": ["kick-sim-win32-x64", "kick-sim.exe"],
});

/** Resolves a Node target to its optional package and executable name. */
export function packageForPlatform(platform, arch) {
  return packages[`${platform}-${arch}`];
}

/** Resolves the installed native binary for a supported Node target. */
export function resolveBinary(platform = process.platform, arch = process.arch) {
  const target = packageForPlatform(platform, arch);
  if (!target) {
    throw new Error(
      `kick-sim does not provide a binary for ${platform}/${arch}. ` +
        "Supported targets are macOS and Linux on x64/arm64, and Windows on x64.",
    );
  }

  const [packageName, executable] = target;
  try {
    return require.resolve(`${packageName}/bin/${executable}`);
  } catch (error) {
    if (error?.code !== "MODULE_NOT_FOUND") throw error;
    throw new Error(
      `The optional package ${packageName} is missing. ` +
        "Reinstall kick-sim without --omit=optional or --no-optional.",
      { cause: error },
    );
  }
}

/** Runs the native binary and forwards termination signals and its exit status. */
export function run(args = process.argv.slice(2)) {
  let binary;
  try {
    binary = resolveBinary();
  } catch (error) {
    console.error(`kick-sim: ${error.message}`);
    process.exitCode = 1;
    return;
  }

  const child = spawn(binary, args, { stdio: "inherit", windowsHide: false });
  const forwardedSignals = ["SIGINT", "SIGTERM", "SIGHUP"];
  for (const signal of forwardedSignals) {
    process.on(signal, () => {
      if (!child.killed) child.kill(signal);
    });
  }

  child.on("error", (error) => {
    console.error(`kick-sim: unable to start ${binary}: ${error.message}`);
    process.exitCode = 1;
  });
  child.on("exit", (code, signal) => {
    if (signal) {
      const signalExitCodes = { SIGHUP: 129, SIGINT: 130, SIGTERM: 143 };
      process.exitCode = signalExitCodes[signal] ?? 1;
      return;
    }
    process.exitCode = code ?? 1;
  });
}

const invokedPath = process.argv[1] ? realpathSync(process.argv[1]) : "";
if (invokedPath === realpathSync(fileURLToPath(import.meta.url))) run();
