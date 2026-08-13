# Decisions (ADRs)

Architecture Decision Records — one markdown per decision. Copy [`_template.md`](_template.md) when recording a new one.

| ID   | Title                                                                  | Status   | Date       |
| ---- | ---------------------------------------------------------------------- | -------- | ---------- |
| 0001 | [Version from git tag](0001-version-from-git-tag.md)                   | Accepted | 2026-05-16 |
| 0002 | [Unified `audit_log` collection](0002-unified-audit-log-collection.md) | Accepted | 2026-05-26 |
| 0003 | [Programmatic xemu input channel](0003-programmatic-xemu-input-channel.md) | Accepted | 2026-07-10 |
| 0004 | [ISO-boot provisioning](0004-iso-boot-provisioning.md)                 | Accepted | 2026-07-10 |

## Status values

`Proposed` | `Accepted` | `Superseded by ADR-XXXX` | `Deprecated`

## Conventions

- Filename: `????-kebab-name.md` (zero-padded to 4 digits).
- ADRs are **immutable once `Accepted`.** Don't edit Context / Decision / Consequences.
- To change direction: write a new ADR that supersedes the old one, and update both Status lines.

See [`../README.md`](../README.md) for the full convention, including how to supersede.
