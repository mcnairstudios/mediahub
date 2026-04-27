# tmdb

TMDB (The Movie Database) API client and image cache. Searches movies and TV series, fetches full detail with cast/crew/genres, and caches results via the `cache/tmdb` package.

## Client

`NewClient(apiKeyFn, cache)` creates a client that reads the API key dynamically (from settings store). All results are cached -- repeat calls for the same query or TMDB ID return from cache without hitting the API.

## ImageCache

`NewImageCache(dir)` creates an HTTP handler that proxies TMDB image URLs through a local disk cache with immutable Cache-Control headers. Query params: `path` (TMDB path), `size` (default w500).

## SyncBatch

Background sync: pass a list of stream names + types, the client searches and caches metadata at 250ms intervals to stay under TMDB rate limits.
