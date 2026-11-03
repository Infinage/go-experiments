# Card Management

Aligned with Constitution v1.0 - Tinder-style swipe review, playful arcade UI.

## Status legend

- ✅ Complete (verified in code)
- ⚠️ Partial
- ❌ Pending

## Epic B-01: Create Flashcard
- **US-B01: Create Flashcard** — ✅ Complete
  - As a logged-in user, I want to create a new flashcard with prompt, answer, and tags.
  - Acceptance: Prompt and answer required, tags optional (comma-separated), card assigned to current user, Datastar patch to cardform with success message.
  - Status: `POST /cards` (+ `?more=1`) in `handlers_cards.go`; smoke-tested in e01.

## Epic B-02: View Home Dashboard
- **US-B02: View Home Dashboard** — ✅ Complete
  - As a logged-in user, I want to see my home dashboard with tag overview and quick actions.
  - Acceptance: User greeting, tag list with due counts, "Add card" and "Import CSV" CTA buttons, mobile-responsive grid layout.
  - Status: `GET /home` in `handlers_home.go`. Note: layout is a stacked list, not a grid; mobile-grid refinement tracked under US-D05.

## Epic B-03: Import Cards from CSV
- **US-B03: Import Cards from CSV** — ✅ Complete
  - As a user, I want to import multiple flashcards from a CSV file so I can quickly populate my deck.
  - Acceptance: CSV must have "prompt" and "answer" columns (required), optional "tags" column (semicolon-separated), error messages for malformed rows, success count displayed.
  - Status: `POST /cards/import` in `handlers_cards.go`; fail-fast parse and BOM handling per e01c.

## Epic B-04: Edit Flashcard
- **US-B04: Edit Flashcard** — ✅ Complete
  - As the card owner, I want to edit an existing flashcard so I can correct mistakes.
  - Acceptance: Only owner can edit, prompt/answer textareas, tag input, Datastar patch with success/error feedback.
  - Status: `GET/POST /cards/{id}/edit` in `handlers_cards.go`; edit form prefilled via server-seeded `data-signals`; owner check against `GetCardsByUser`; tags replaced via `Store.UpdateCard`.

## Epic B-05: Delete Flashcard
- **US-B05: Delete Flashcard** — ✅ Complete
  - As the card owner, I want to delete flashcards I no longer need.
  - Acceptance: Permanent deletion of card and associated stats, confirmation feedback, Datastar update to home dashboard.
  - Status: `DELETE /cards/{id}` in `handlers_cards.go`; deletion goes through `Store.DeleteCards` (card + stats + tags). Confirmation is a client-side two-step inline confirm in the cards list; the list (not the home dashboard) re-patches after delete, since the entry point is the "Your cards" view.

## Epic B-06: Bulk Delete Cards
- **US-B06: Bulk Delete Cards** — ✅ Complete (added per user request)
  - As a user with a large deck, I want to select multiple cards and delete them in one action.
  - Acceptance: Checkbox per card row, a "Delete selected" action deletes all selected cards (and their stats/tags) in one request, and cards owned by other users are silently filtered out of the selection.
  - Status: `DELETE /cards` in `handlers_cards.go` (`parseBulkIDs` + `ownedIDs`, unit-tested); selectable checkboxes bind `pick_<id>` boolean signals.