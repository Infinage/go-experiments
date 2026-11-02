# Card Management

Aligned with Constitution v1.0 - Tinder-style swipe review, playful arcade UI.

## Epic B-01: Create Flashcard
- **US-B01: Create Flashcard**
  - As a logged-in user, I want to create a new flashcard with prompt, answer, and tags.
  - Acceptance: Prompt and answer required, tags optional (comma-separated), card assigned to current user, Datastar patch to cardform with success message.

## Epic B-02: View Home Dashboard
- **US-B02: View Home Dashboard**
  - As a logged-in user, I want to see my home dashboard with tag overview and quick actions.
  - Acceptance: User greeting, tag list with due counts, "Add card" and "Import CSV" CTA buttons, mobile-responsive grid layout.

## Epic B-03: Import Cards from CSV
- **US-B03: Import Cards from CSV**
  - As a user, I want to import multiple flashcards from a CSV file so I can quickly populate my deck.
  - Acceptance: CSV must have "prompt" and "answer" columns (required), optional "tags" column (semicolon-separated), error messages for malformed rows, success count displayed.

## Epic B-04: Edit Flashcard
- **US-B04: Edit Flashcard**
  - As the card owner, I want to edit an existing flashcard so I can correct mistakes.
  - Acceptance: Only owner can edit, prompt/answer textareas, tag input, Datastar patch with success/error feedback.

## Epic B-05: Delete Flashcard
- **US-B05: Delete Flashcard**
  - As the card owner, I want to delete flashcards I no longer need.
  - Acceptance: Permanent deletion of card and associated stats, confirmation feedback, Datastar update to home dashboard.