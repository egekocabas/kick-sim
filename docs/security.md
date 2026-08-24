# Security model

Kick Sim is a local development tool with access to project files, a simulator private key, loopback HTTP receivers, and retained response bodies. It is not designed for remote hosting or production trust.

## Trust boundaries

- Studio binds only to an explicit loopback IP and validates the exact `Host` header.
- A random per-process control token protects every mutating API request. Browser requests receive it only in an `HttpOnly`, `SameSite=Strict` cookie; authenticated non-browser callers use a bearer token.
- Browser mutations require the exact Studio origin and JSON media type. Missing-origin mutations require bearer authentication.
- Studio does not enable permissive CORS, rejects framing, applies a restrictive content security policy, disables response caching for API data, and bounds request bodies and HTTP timeouts.
- Webhook delivery accepts only HTTP(S) loopback destinations. DNS results are checked before connecting, redirects are denied, and credentials or URL fragments are rejected.

## Keys and history

The simulator private key can forge events for any environment that trusts its public key. Never configure the simulator key in production. Signing remains in Go; the browser can read only the public key and fingerprint. Private keys are created with restrictive permissions where supported and are never returned by Studio APIs.

History can contain receiver headers and bodies. Storage is bounded by response-size, retention-age, and run-count settings, and can be disabled. `.runtime/` is local runtime state and is ignored by initialized workspaces.

## Files and templates

Authored IDs cannot escape their designated directories. Scenario writes reject symlink paths, use same-directory temporary files, synchronize data, and atomically rename. Dynamic templates are a closed expression set; they cannot execute code, run shells, read files, or access the network.

Security reports should include the affected version, reproduction steps, and impact without attaching real Kick credentials, production private keys, or sensitive webhook payloads.
