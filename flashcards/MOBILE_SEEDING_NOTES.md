# Mobile/PW seeding notes (decided during Wails migration setup)

- applicationId: **com.infinage.flashcards** (user has no company name yet; owns domain
  `infinage.space` — future web host of the web version; `com.infinage` prefix is
  acceptable long-term).
- Device DB: starts **pre-seeded via adb push** (easier to test than empty-first-launch);
  fixture: the same sample dataset used by unit tests, built into a small SQLite file,
  pushed into `/data/data/com.infinage.flashcards/files/flashcards.db`.
- TODO (later): an optional CLI/script to seed data on BOTH web and mobile builds —
  e.g. `make seed` for `flashcards.db` working-tree copy plus `seed-android.sh` wrapping
  the adb push.
- On-device checklist order: counters/regressions first, media (camera/record) second.
- Git flow: working tree until green, then commit.
