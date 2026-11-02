# ADR 0021: Collections in the UI

- **Status:** accepted
- **Date:** 2026-11-02
- Uses the schema and repository from ADR 0015.

## Decision

### Bound methods (`app_collections.go`)

`ListCollections`, `CreateCollection`, `RenameCollection`,
`DeleteCollection`, `AddToCollection`, `RemoveFromCollection` and
`ImageCollections`. `ListLibrary(collectionID, offset, limit)` gained the
collection (0 = whole library). Library errors map to `invalid_input`
messages: name taken (case-insensitive), name not 1–100 characters,
collection or image no longer exists. `ImageCollections` returns `[]`,
never `null`, so the frontend needs no special case.

### Library view

- A row of chips above the gallery: **All images** with the library count,
  one chip per collection with its image count (`aria-pressed` marks the
  shown one), and **+ New collection**. Creating a collection switches to it.
- For the shown collection: **Rename** and **Delete collection…**. Deleting
  asks for confirmation and says that the images stay in the library; focus
  starts on Keep.
- If the shown collection disappears (deleted), the view returns to All
  images.

### Preview

- In the library preview, a **Collections** group lists every collection
  with a checkbox for this image, and **+ New collection** creates one and
  adds the image in one step.
- Unticking the collection that is being shown removes the image from the
  gallery and moves the preview to the next image; the image stays in the
  library.

### Names

`NameForm` checks names before sending them, with the same rules as the
database (whitespace collapsed, 1–100 characters counted as characters,
unique case-insensitively, a collection may keep its own name in another
case). The backend still validates; its message is shown if it refuses.
Escape cancels the form without closing the preview dialog around it.

## Limitations

- Images can only be added to collections from the library, not directly
  from search results; downloading first is required.
- No ordering inside a collection other than newest first.
