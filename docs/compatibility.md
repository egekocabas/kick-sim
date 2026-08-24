# Compatibility and provenance

Kick Sim models reviewed event contracts from a pinned revision of `KickEngineering/KickDevDocs`. `kick-sim compatibility` reports the full upstream commit, retrieval date, modeled documents, supported event versions, known differences, and a digest covering every bundled schema and default payload.

The simulator signs with its workspace key, never Kick's production key. A successful Kick Sim test demonstrates compatibility with the modeled and pinned contract; it is not evidence that a request originated from Kick.

Upstream documentation changes require human review. Updating the pin means reviewing the relevant diff, changing schemas/defaults and tests intentionally, updating known differences, and publishing a new Kick Sim release. Automation must not silently rewrite protocol fixtures.

Receivers should tolerate unknown additive payload fields while validating the fields they depend on. Kick Sim schemas remain strict so fixture mistakes are caught locally, but they should not encourage fragile production parsing.
