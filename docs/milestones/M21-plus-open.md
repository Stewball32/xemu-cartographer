# Milestone 21+ — Open

- **qcow2 disk-image editing.** Long shot. Easiest path may be running an FTP server inside the xemu container so UnleashX's FTP client can drop files onto the guest disk. Alternative: mount qcow2 host-side via libguestfs / `qemu-nbd` and edit FATX directly. Investigate before committing to either.
- **POV marker overlay (M12 fallback target).** If M12 turns out infeasible (perspective math too brittle, camera offsets unreliable), demote it here as a research item rather than a committed milestone.
- Second-game generalization test (confirm the scraper registry abstraction holds for something non-Halo) — partially answered by M20 but stays open for a third title.
- Community-contributed offset tables (moderation workflow).
- Post-game report UI (replaces HaloCaster's Excel export) — could land alongside M15 stats as a follow-up.
- Hosted / remote deployment story.
