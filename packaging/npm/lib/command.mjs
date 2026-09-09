import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";

export function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: "utf8", ...options });
  if (result.error) {
    throw new Error(
      `Unable to start ${command}: ${result.error.message}`,
      { cause: result.error },
    );
  }
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed ` +
        `(${result.signal ? `signal ${result.signal}` : `exit ${result.status}`}):\n` +
        `${result.stdout ?? ""}\n${result.stderr ?? ""}`,
    );
  }
  return result.stdout;
}

export function runNpm(args, options = {}) {
  if (process.platform !== "win32") return run("npm", args, options);

  // npm.cmd cannot be spawned directly. Resolve the active npm installation
  // through PATH so a globally pinned npm takes precedence over Node's bundle.
  const shim = run("where.exe", ["npm.cmd"], options).trim().split(/\r?\n/)[0];
  const cli = resolve(dirname(shim), "node_modules/npm/bin/npm-cli.js");
  if (!existsSync(cli)) {
    throw new Error(`Cannot locate npm CLI beside ${shim}: ${cli}`);
  }
  // Keep paths and arguments separate, including paths containing spaces.
  return run(process.execPath, [cli, ...args], options);
}
