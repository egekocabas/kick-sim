export const platforms = Object.freeze([
  {
    goos: "darwin",
    goarch: "amd64",
    nodePlatform: "darwin",
    nodeArch: "x64",
    packageName: "kick-sim-darwin-x64",
    executable: "kick-sim",
  },
  {
    goos: "darwin",
    goarch: "arm64",
    nodePlatform: "darwin",
    nodeArch: "arm64",
    packageName: "kick-sim-darwin-arm64",
    executable: "kick-sim",
  },
  {
    goos: "linux",
    goarch: "amd64",
    nodePlatform: "linux",
    nodeArch: "x64",
    packageName: "kick-sim-linux-x64",
    executable: "kick-sim",
  },
  {
    goos: "linux",
    goarch: "arm64",
    nodePlatform: "linux",
    nodeArch: "arm64",
    packageName: "kick-sim-linux-arm64",
    executable: "kick-sim",
  },
  {
    goos: "windows",
    goarch: "amd64",
    nodePlatform: "win32",
    nodeArch: "x64",
    packageName: "kick-sim-win32-x64",
    executable: "kick-sim.exe",
  },
]);

export function platformForNode(nodePlatform, nodeArch) {
  return platforms.find(
    (platform) =>
      platform.nodePlatform === nodePlatform && platform.nodeArch === nodeArch,
  );
}

export function platformForGo(goos, goarch) {
  return platforms.find(
    (platform) => platform.goos === goos && platform.goarch === goarch,
  );
}
