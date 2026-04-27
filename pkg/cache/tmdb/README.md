# cache/tmdb

In-memory cache for TMDB (The Movie Database) metadata. Stores movie and series information including posters, synopsis, cast, genres, and ratings.

Implements the `cache.Cache` interface for registry integration, plus typed helpers (`GetMovie`, `SetMovie`, `GetSeries`, `SetSeries`) for direct access without type assertions.

No HTTP client included — this package handles storage only. A TMDB API client will be added separately.
