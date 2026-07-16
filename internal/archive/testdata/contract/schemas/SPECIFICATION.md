# PkuHole Archive Contract 2.1.0

## Container

An archive is a standard ZIP file with these base entries:

```text
manifest.json     required
data.json         required
readable.txt      optional, informational only
media/index.json  optional media extension v1
media/*           optional content-addressed media
```

Readers validate normalized forward-slash paths, reject absolute paths,
traversal, duplicates, decompression limit violations, and undeclared media.
`manifest.json` and `data.json` are UTF-8 JSON. Import behavior never depends
on `readable.txt`.

Contract 2.1 writers use ZIP method `STORE` for portable entries. This keeps
the reference browser reader synchronous and makes already-compressed media
deterministic without a second compression layer. Readers may additionally
accept `DEFLATE` archives from older or third-party writers, but must always
accept `STORE`.

## Versions and producers

- `schemaVersion: 2` selects the compatible v2 family.
- `specVersion` identifies this precise revision and is optional only for
  legacy v2.0 archives.
- `toolVersion` remains required for v2.0 compatibility.
- `producer` names the application that wrote the archive. It does not grant
  trust and must not contain a user, device, or account identifier.

Readers decide trust from local policy and content validation, never from the
producer name.

## Snapshot semantics

Each item is one post snapshot plus the comments captured for that post.

- `pid` is the portable string key and must agree with `hole.pid`.
- `source` records why the post belongs to the snapshot.
- `source: referenced` is context-only. Consumers may archive and display it,
  but must not convert it into a follow or other remote write.
- `fetchStatus: ok` means the producer considers the captured record complete
  for the requested options.
- `fetchStatus: partial` means missing fields or comments are unknown. A
  consumer may merge present data but must not interpret absence as deletion.
- Duplicate PIDs are invalid because their merge order would be ambiguous.
- Invalid comments may be reported and skipped only when the consumer exposes
  a partial preflight result before any write.

`manifest.complete` describes the whole requested export. Per-item
`fetchStatus` remains authoritative for an individual item.

## Identity and idempotence

The SHA-256 of the complete archive is the primary import identity. `runId` is
producer-scoped audit metadata and supports duplicate detection, but it is not
a globally stable content identifier. Rewriting content should create a new
run ID.

## Extensions

`manifest.extensions` declares present extensions. Unknown optional
extensions are ignored without discarding base post/comment data. A reader
rejects an extension listed in `requiredExtensions` when it does not support
that name and version.

Extensions may add optional manifest or item properties. New ZIP roots require
a prior compatible reader release; v2 currently reserves only `media/`.

The Studio sources extension stores portable provenance only:

```json
{ "source": "followed", "sourceRef": "producer-local-reference", "contextOnly": false }
```

`source` uses `followed`, `explicit`, `referenced`, or `legacy-v1`. `sourceRef` is
an optional opaque producer-local reference of at most 128 characters; it is
not an account or device identity. `contextOnly` is always effectively true
for `referenced`. Database column names, timestamps, and local primary keys are
not part of this extension. Writers should merge repeated semantic sources and
must not build a transitive history by re-exporting prior archive hashes. A
reader processes at most 16 source entries per item.

## Privacy profile

Portable archives must not include:

- login cookies, bearer tokens, authorization headers, or passwords;
- raw login UUID credentials;
- account fingerprints or another stable identifier created only to link
  exports to the same user;
- local filesystem paths.

Content supplied by Treehole may still be sensitive. The archive is not
encrypted and relies on the user's filesystem and device protection.

## Consumer safety

Consumers perform a read-only preflight before mutation. Limits are part of
the implementation contract: current reference readers cap the compressed
archive at 200 MiB, expanded data at 500 MiB, individual media at 50 MiB, and
items at 20,000. A consumer may choose lower limits but must report them as a
capability instead of silently truncating.
