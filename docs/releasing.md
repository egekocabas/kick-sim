# Release and package publishing

GitHub Releases are the source of truth for every distributed `kick-sim` executable. GoReleaser builds the embedded Studio once, creates the platform archives and checksums, and generates the Homebrew Cask and Scoop manifest. The npm, Homebrew, and Scoop publishing jobs redistribute those exact artifacts without rebuilding them.

## One-time setup

Complete this setup before pushing the first tag that uses the multi-channel workflow.

1. Create the public repositories `egekocabas/homebrew-tap` and `egekocabas/scoop-bucket`, each with a `main` branch and a minimal README.
2. Create a fine-grained GitHub token with contents read/write access only to those two repositories. Add it to `egekocabas/kick-sim` as the Actions secret `PACKAGE_REPOSITORIES_TOKEN`.
3. Run a release build without publishing, then use the generated tarballs in `dist/distribution/npm/tarballs` to interactively publish the five platform packages followed by `kick-sim`. The first publish reserves the package names because npm trusted publishing can only be configured for packages that already exist.
4. For each of these packages, configure the GitHub trusted publisher for repository `egekocabas/kick-sim`, workflow file `release.yml`, and the `npm publish` permission:
   - `kick-sim-darwin-x64`
   - `kick-sim-darwin-arm64`
   - `kick-sim-linux-x64`
   - `kick-sim-linux-arm64`
   - `kick-sim-win32-x64`
   - `kick-sim`
5. After a successful OIDC release, set each npm package's publishing access to require 2FA and disallow token publishing.

The npm package metadata must retain the exact repository URL `https://github.com/egekocabas/kick-sim`; npm uses it when validating trusted publishing provenance.

## Repository protection

Protect `main` with pull requests and the single `Required CI` status check. The CI
workflow intentionally starts for every pull request so documentation-only changes
cannot leave the required check permanently pending. `Required CI` succeeds only
after the quality, cross-platform test, Studio, release snapshot, and package smoke
jobs have all succeeded.

Protect release tags with an active tag ruleset targeting `v*`. Restrict tag
creation, updates, and deletions; block force pushes; and allow only repository
administrators to bypass the rules so new versions can be created. Treat every
published release tag as immutable and never move it to another commit.

## Creating a release

Create releases from a tested commit on `main`:

```sh
git tag -a v0.5.0 -m "v0.5.0"
git push origin v0.5.0
```

The release workflow performs these stages:

1. Test the Go source and build the Studio bundle.
2. Run GoReleaser and publish the GitHub Release, archives, and `checksums.txt`.
3. Create one retained Actions artifact containing the npm packages, Homebrew Cask, Scoop manifest, and release metadata.
4. Publish the package-manager outputs in independent jobs. The five platform npm packages become available before the `kick-sim` launcher is published.
5. Install the npm package from the registry on Linux, macOS, and Windows; verify its version, workspace commands, and embedded Studio.

A plain semantic version publishes npm's `latest` tag and updates the Homebrew and Scoop repositories. A prerelease such as `v0.6.0-rc.1` publishes npm's `next` tag and skips Homebrew and Scoop. Prereleases never move npm's `latest` tag.

## Local verification

Build the Studio before creating a GoReleaser snapshot:

```sh
npm ci --prefix web
make web
goreleaser check
goreleaser release --snapshot --clean --skip=publish
node packaging/npm/scripts/build.mjs --dist dist --out dist/distribution/npm
node packaging/release/collect.mjs --dist dist --out dist/distribution
npm test --prefix packaging/npm
node packaging/npm/scripts/smoke.mjs --bundle dist/distribution/npm
```

The snapshot must contain six npm tarballs, `homebrew/kick-sim.rb`, `scoop/kick-sim.json`, and matching version metadata. No command in this verification sequence publishes externally.

## Failure recovery

Publisher jobs are idempotent. Re-run only failed jobs from the GitHub Actions run: an npm package with matching registry integrity is skipped, an unchanged tap or bucket manifest is a no-op, and remaining outputs continue. Publishing stops rather than overwriting an existing package version with different content or moving a channel to an older version.

Released package versions and Git tags are immutable. For a bad release:

1. Fix the defect and publish a new patch version.
2. Deprecate the affected npm version with a message directing users to the patch.
3. Allow the new release to advance the Homebrew Cask and Scoop manifest.

Do not retag a commit, replace npm tarballs, or edit a Cask or Scoop manifest to point at different bytes for an existing version.
