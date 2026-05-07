//! Trailers WASM plugin for MediaHub.
//!
//! Fetches upcoming and now-playing movies from TMDB, resolves their YouTube
//! trailer URLs, and returns them as playable streams.

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::alloc::Layout;

// ---------------------------------------------------------------------------
// Host function imports (provided by the MediaHub WASM host via "env" module)
// ---------------------------------------------------------------------------

#[link(wasm_import_module = "env")]
extern "C" {
    /// Log a message. level: 0=debug, 1=info, 2=warn, 3=error.
    fn host_log(level: i32, msg_ptr: i32, msg_len: i32);

    /// Make an HTTP request. Returns a packed i64: high32=ptr, low32=len of the
    /// response body written into WASM memory. Returns 0 on error.
    fn host_http_request(
        url_ptr: i32,
        url_len: i32,
        method_ptr: i32,
        method_len: i32,
        headers_ptr: i32,
        headers_len: i32,
        body_ptr: i32,
        body_len: i32,
    ) -> i64;
}

// ---------------------------------------------------------------------------
// Logging helpers
// ---------------------------------------------------------------------------

fn log_info(msg: &str) {
    unsafe {
        host_log(1, msg.as_ptr() as i32, msg.len() as i32);
    }
}

fn log_error(msg: &str) {
    unsafe {
        host_log(3, msg.as_ptr() as i32, msg.len() as i32);
    }
}

// ---------------------------------------------------------------------------
// HTTP helper
// ---------------------------------------------------------------------------

fn http_get(url: &str) -> Option<Vec<u8>> {
    let method = "GET";
    let packed = unsafe {
        host_http_request(
            url.as_ptr() as i32,
            url.len() as i32,
            method.as_ptr() as i32,
            method.len() as i32,
            0, // no headers
            0,
            0, // no body
            0,
        )
    };
    if packed == 0 {
        return None;
    }
    let ptr = (packed >> 32) as u32;
    let len = (packed & 0xFFFF_FFFF) as u32;
    if len == 0 {
        return None;
    }
    let slice = unsafe { std::slice::from_raw_parts(ptr as *const u8, len as usize) };
    Some(slice.to_vec())
}

// ---------------------------------------------------------------------------
// Memory allocator export (required by the host)
// ---------------------------------------------------------------------------

/// Allocate `size` bytes in WASM linear memory and return the pointer.
#[no_mangle]
pub extern "C" fn alloc(size: i32) -> i32 {
    if size <= 0 {
        return 0;
    }
    let layout = match Layout::from_size_align(size as usize, 1) {
        Ok(l) => l,
        Err(_) => return 0,
    };
    let ptr = unsafe { std::alloc::alloc(layout) };
    if ptr.is_null() {
        return 0;
    }
    ptr as i32
}

/// Pack a pointer and length into a single i64 (high32=ptr, low32=len).
fn pack_ptr_len(ptr: *const u8, len: usize) -> i64 {
    ((ptr as i64) << 32) | (len as i64)
}

/// Allocate a copy of `data` in WASM memory and return a packed ptr+len i64.
fn return_bytes(data: &[u8]) -> i64 {
    if data.is_empty() {
        return 0;
    }
    let layout = match Layout::from_size_align(data.len(), 1) {
        Ok(l) => l,
        Err(_) => return 0,
    };
    let ptr = unsafe { std::alloc::alloc(layout) };
    if ptr.is_null() {
        return 0;
    }
    unsafe {
        std::ptr::copy_nonoverlapping(data.as_ptr(), ptr, data.len());
    }
    pack_ptr_len(ptr, data.len())
}

/// Convenience: serialize `value` to JSON, allocate it, and return packed i64.
fn return_json<T: Serialize>(value: &T) -> i64 {
    match serde_json::to_vec(value) {
        Ok(bytes) => return_bytes(&bytes),
        Err(_) => 0,
    }
}

// ---------------------------------------------------------------------------
// Plugin descriptor (returned by `describe`)
// ---------------------------------------------------------------------------

#[derive(Serialize)]
struct PluginDescriptor {
    r#type: &'static str,
    label: &'static str,
    short_label: &'static str,
    color: &'static str,
    icon: &'static str,
    version: &'static str,
    description: &'static str,
    config_fields: Vec<ConfigField>,
}

#[derive(Serialize)]
struct ConfigField {
    key: &'static str,
    label: &'static str,
    r#type: &'static str,
    required: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    placeholder: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    help_text: Option<&'static str>,
}

// ---------------------------------------------------------------------------
// TMDB API types
// ---------------------------------------------------------------------------

#[derive(Deserialize)]
struct TmdbMovieList {
    results: Vec<TmdbMovie>,
}

#[derive(Deserialize)]
struct TmdbMovie {
    id: i64,
    title: Option<String>,
    #[allow(dead_code)]
    overview: Option<String>,
    poster_path: Option<String>,
    #[allow(dead_code)]
    release_date: Option<String>,
    #[allow(dead_code)]
    vote_average: Option<f64>,
}

#[derive(Deserialize)]
struct TmdbVideoList {
    results: Vec<TmdbVideo>,
}

#[derive(Deserialize)]
struct TmdbVideo {
    key: String,
    site: String,
    r#type: String,
    name: String,
}

// ---------------------------------------------------------------------------
// Refresh config (JSON passed by the host)
// ---------------------------------------------------------------------------

#[derive(Deserialize)]
struct RefreshConfig {
    #[serde(default)]
    tmdb_key: String,
    #[serde(default)]
    source_id: String,
}

// ---------------------------------------------------------------------------
// Refresh result (returned to the host)
// ---------------------------------------------------------------------------

#[derive(Serialize)]
struct RefreshResult {
    streams: Vec<PluginStream>,
}

#[derive(Serialize)]
struct PluginStream {
    name: String,
    url: String,
    group: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    logo: Option<String>,
    vod_type: String,
}

// ---------------------------------------------------------------------------
// Deterministic stream ID (matches Go implementation)
// ---------------------------------------------------------------------------

fn deterministic_stream_id(source_id: &str, key: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(format!("{}:{}", source_id, key));
    let result = hasher.finalize();
    // First 16 bytes = 32 hex chars, matching Go's fmt.Sprintf("%x", h[:16])
    hex::encode(&result[..16])
}

// ---------------------------------------------------------------------------
// TMDB fetching logic
// ---------------------------------------------------------------------------

const TMDB_BASE: &str = "https://api.themoviedb.org/3";

fn fetch_movie_list(endpoint: &str, api_key: &str) -> Vec<TmdbMovie> {
    let url = format!("{}/{}?api_key={}&page=1", TMDB_BASE, endpoint, api_key);
    let Some(body) = http_get(&url) else {
        log_error(&format!("trailers: failed to fetch {}", endpoint));
        return Vec::new();
    };
    match serde_json::from_slice::<TmdbMovieList>(&body) {
        Ok(list) => list.results,
        Err(e) => {
            log_error(&format!("trailers: failed to parse {}: {}", endpoint, e));
            Vec::new()
        }
    }
}

fn find_trailer(tmdb_id: i64, api_key: &str) -> Option<(String, String)> {
    let url = format!("{}/movie/{}/videos?api_key={}", TMDB_BASE, tmdb_id, api_key);
    let body = http_get(&url)?;
    let videos: TmdbVideoList = serde_json::from_slice(&body).ok()?;

    // Prefer "Trailer" type, fall back to "Teaser"
    for v in &videos.results {
        if v.site == "YouTube" && v.r#type == "Trailer" {
            let trailer_url = format!("https://www.youtube.com/watch?v={}", v.key);
            return Some((trailer_url, v.name.clone()));
        }
    }
    for v in &videos.results {
        if v.site == "YouTube" && v.r#type == "Teaser" {
            let trailer_url = format!("https://www.youtube.com/watch?v={}", v.key);
            return Some((trailer_url, v.name.clone()));
        }
    }
    None
}

// ---------------------------------------------------------------------------
// Exported WASM functions
// ---------------------------------------------------------------------------

/// Return the plugin descriptor as JSON.
#[no_mangle]
pub extern "C" fn describe() -> i64 {
    let desc = PluginDescriptor {
        r#type: "trailers",
        label: "Trailers",
        short_label: "TRL",
        color: "#E91E63",
        icon: "movie",
        version: "0.1.0",
        description: "TMDB movie trailers (upcoming and now playing)",
        config_fields: vec![
            ConfigField {
                key: "tmdb_key",
                label: "TMDB API Key",
                r#type: "password",
                required: true,
                placeholder: Some("Enter your TMDB v3 API key"),
                help_text: Some("Get a free key at https://www.themoviedb.org/settings/api"),
            },
        ],
    };
    return_json(&desc)
}

/// Refresh: fetch upcoming and now-playing movies from TMDB, resolve trailers.
/// Input: JSON config with `tmdb_key` and `source_id`.
/// Output: JSON `{ "streams": [...] }`.
#[no_mangle]
pub extern "C" fn refresh(config_ptr: i32, config_len: i32) -> i64 {
    // Read config from WASM memory.
    let config_bytes =
        unsafe { std::slice::from_raw_parts(config_ptr as *const u8, config_len as usize) };

    let config: RefreshConfig = match serde_json::from_slice(config_bytes) {
        Ok(c) => c,
        Err(e) => {
            log_error(&format!("trailers: failed to parse config: {}", e));
            return return_json(&RefreshResult {
                streams: Vec::new(),
            });
        }
    };

    if config.tmdb_key.is_empty() {
        log_error("trailers: no TMDB API key configured");
        return return_json(&RefreshResult {
            streams: Vec::new(),
        });
    }

    log_info(&format!("trailers: refreshing with source_id={}", config.source_id));

    // Fetch upcoming movies.
    let mut movies = fetch_movie_list("movie/upcoming", &config.tmdb_key);

    // Fetch now-playing and append (errors are non-fatal).
    let now_playing = fetch_movie_list("movie/now_playing", &config.tmdb_key);
    movies.extend(now_playing);

    log_info(&format!("trailers: found {} movies", movies.len()));

    let mut streams = Vec::new();
    let mut seen = std::collections::HashSet::new();

    for movie in &movies {
        let title = match &movie.title {
            Some(t) if !t.is_empty() => t,
            _ => continue,
        };

        let Some((trailer_url, trailer_name)) = find_trailer(movie.id, &config.tmdb_key) else {
            continue;
        };

        // Deterministic ID uses source_id + TMDB movie ID (matching Go implementation).
        let id = deterministic_stream_id(&config.source_id, &movie.id.to_string());
        if !seen.insert(id) {
            continue;
        }

        let logo = movie
            .poster_path
            .as_ref()
            .map(|p| format!("https://image.tmdb.org/t/p/w500{}", p));

        let name = format!("{} - {}", title, trailer_name);

        streams.push(PluginStream {
            name,
            url: trailer_url,
            group: "Trailers".to_string(),
            logo,
            vod_type: "movie".to_string(),
        });
    }

    log_info(&format!("trailers: returning {} streams", streams.len()));

    return_json(&RefreshResult { streams })
}

/// Handle user interactions (not used by trailers, but required export).
#[no_mangle]
pub extern "C" fn interact(action_ptr: i32, action_len: i32) -> i64 {
    let _ = (action_ptr, action_len);
    return_json(&serde_json::json!({"error": "no interactions supported"}))
}
