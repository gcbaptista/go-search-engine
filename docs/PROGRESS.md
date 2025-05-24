📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

# Project Progress: Go Search Engine

This document tracks the implementation progress based on the initial plan.

## VI. Next Steps / Implementation Phases

- [x] **1. Core Data Structures:** Implement `Document`, `IndexSettings`, `PostingEntry`, `InvertedIndex`,
      `DocumentStore`.
  - `model/document.go` created.
  - `config/settings.go` created.
  - `index/posting.go` created.
  - `index/inverted_index.go` created.
  - `store/document_store.go` created.
- [x] **2. Tokenizer:** Create a basic tokenizer.
  - `internal/tokenizer/tokenizer.go` created with `Tokenize` function.
  - _(Future refinement for "theoffice" case noted)_
- [x] **3. IndexerService:** Implement `AddDocument` logic, including tokenization and updating `InvertedIndex` (with
      sorted posting lists) and `DocumentStore`.
  - `internal/indexing/service.go` created with `Service.AddDocuments()`.
  - Reflection used for accessing document fields.
  - Posting lists are sorted by score (popularity).
- [x] **4. SearchService (Basic):** Implement search with exact term match, fetching from `DocumentStore`, and sorting
      by popularity. No filters or typos yet.
  - `internal/search/service.go` created (placeholder).
- [x] **5. REST API (Basic):** Expose index creation, document addition, and basic search.
  - `api/` directory exists.
- [x] **6. Typo Tolerance:** Implement Levenshtein and integrate into `SearchService`.
  - `internal/typoutil/` directory exists for typo generation logic.
- [x] **7. Filters:** Implement filter logic in `SearchService`.
- [x] **8. "theoffice" refinement:** Improve tokenizer or add specific logic for compound word splitting if needed.
- [x] **9. Concurrency & Persistence:**
  - [x] Add `sync.RWMutex` for concurrent access to shared data (`InvertedIndex`, `DocumentStore`). (Done for core
        structures and Engine)
  - [x] Consider persistence (e.g., saving index to disk using Gob encoding, BadgerDB, or BoltDB).
- [ ] **10. Testing:** Write unit tests for each component (tokenizer, indexer, searcher) and integration tests for the
      API.

## Additional Architectural Components Implemented:

- [x] **Engine (Orchestrator):** Manages multiple named indexes.
  - `internal/engine/instance.go` created for `IndexInstance` (implements `services.IndexAccessor`).
  - `internal/engine/engine.go` created for `Engine` (implements `services.IndexManager`).
  - Handles creation, retrieval, deletion, and listing of indexes.
- [x] **Service Interfaces:** Defined for clear component boundaries.
  - `services/interfaces.go` created with `Indexer`, `Searcher`, `IndexManager`, `IndexAccessor`, and supporting
    structs.

---

Next up: **Testing**
