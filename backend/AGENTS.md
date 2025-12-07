# Coding Agent Instructions

- This backend exposes a small HTTPS API via chi middleware and the LZC SDK; keep handlers short, reuse the existing router wiring in `main.go`, and avoid introducing new global state.
- The LPK registry writes BoltDB and `.lpk` artifacts beneath `/lzcapp/var/lpks`; treat that layout as a stable contract so older uploads remain readable after code changes.
- `LPKStore.Save` streams multipart uploads to disk while hashing; preserve the streaming behavior so large packages do not require extra buffering.
- Any change to API payloads or error codes should note how Terraform provider clients are expected to interact with the new shape.
