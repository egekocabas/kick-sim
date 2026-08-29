import { extname } from "node:path";

import { parseSemver } from "../../npm/lib/semver.mjs";

export function manifestVersion(path, contents) {
  if (extname(path) === ".rb") {
    const match = /^\s*version\s+"([^"]+)"/m.exec(contents);
    if (!match) throw new Error(`Homebrew Cask has no version stanza: ${path}`);
    parseSemver(match[1]);
    return match[1];
  }
  if (extname(path) === ".json") {
    const version = JSON.parse(contents).version;
    if (typeof version !== "string") {
      throw new Error(`Scoop manifest has no string version: ${path}`);
    }
    parseSemver(version);
    return version;
  }
  throw new Error(`unsupported package manifest: ${path}`);
}
