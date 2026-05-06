use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::slice;

// ============================================================
// Host function imports
// ============================================================

extern "C" {
    fn host_log(level: u32, msg_ptr: u32, msg_len: u32);
    fn host_http_request(
        url_ptr: u32, url_len: u32,
        method_ptr: u32, method_len: u32,
        headers_ptr: u32, headers_len: u32,
        body_ptr: u32, body_len: u32,
    ) -> u64;
    fn host_kv_get(key_ptr: u32, key_len: u32) -> u64;
    fn host_kv_set(key_ptr: u32, key_len: u32, val_ptr: u32, val_len: u32);
}

// ============================================================
// Memory management exports
// ============================================================

#[no_mangle]
pub extern "C" fn alloc(size: u32) -> u32 {
    let layout = std::alloc::Layout::from_size_align(size as usize, 1).unwrap();
    unsafe { std::alloc::alloc(layout) as u32 }
}

#[no_mangle]
pub extern "C" fn dealloc(ptr: u32, size: u32) {
    let layout = std::alloc::Layout::from_size_align(size as usize, 1).unwrap();
    unsafe { std::alloc::dealloc(ptr as *mut u8, layout) }
}

// ============================================================
// Helpers
// ============================================================

fn pack_ptr_len(ptr: u32, len: u32) -> u64 {
    ((ptr as u64) << 32) | (len as u64)
}

fn unpack_ptr_len(packed: u64) -> (u32, u32) {
    let ptr = (packed >> 32) as u32;
    let len = (packed & 0xFFFFFFFF) as u32;
    (ptr, len)
}

fn read_input(ptr: u32, len: u32) -> Vec<u8> {
    unsafe { slice::from_raw_parts(ptr as *const u8, len as usize).to_vec() }
}

fn return_json<T: Serialize>(value: &T) -> u64 {
    match serde_json::to_vec(value) {
        Ok(data) => {
            let ptr = data.as_ptr() as u32;
            let len = data.len() as u32;
            std::mem::forget(data);
            pack_ptr_len(ptr, len)
        }
        Err(_) => 0,
    }
}

fn log_info(msg: &str) {
    let bytes = msg.as_bytes();
    unsafe { host_log(1, bytes.as_ptr() as u32, bytes.len() as u32) }
}

fn log_warn(msg: &str) {
    let bytes = msg.as_bytes();
    unsafe { host_log(2, bytes.as_ptr() as u32, bytes.len() as u32) }
}

fn http_get_with_headers(url: &str, headers_json: &str) -> Option<Vec<u8>> {
    let url_bytes = url.as_bytes();
    let method = b"GET";
    let headers = headers_json.as_bytes();
    let body = b"";

    let result = unsafe {
        host_http_request(
            url_bytes.as_ptr() as u32, url_bytes.len() as u32,
            method.as_ptr() as u32, method.len() as u32,
            headers.as_ptr() as u32, headers.len() as u32,
            body.as_ptr() as u32, body.len() as u32,
        )
    };

    if result == 0 {
        return None;
    }

    let (ptr, len) = unpack_ptr_len(result);
    if len == 0 {
        return None;
    }

    Some(unsafe { slice::from_raw_parts(ptr as *const u8, len as usize).to_vec() })
}

#[allow(dead_code)]
fn kv_get(key: &str) -> Option<String> {
    let kb = key.as_bytes();
    let result = unsafe { host_kv_get(kb.as_ptr() as u32, kb.len() as u32) };
    if result == 0 {
        return None;
    }
    let (ptr, len) = unpack_ptr_len(result);
    if len == 0 {
        return None;
    }
    let data = unsafe { slice::from_raw_parts(ptr as *const u8, len as usize) };
    Some(String::from_utf8_lossy(data).to_string())
}

#[allow(dead_code)]
fn kv_set(key: &str, value: &str) {
    let kb = key.as_bytes();
    let vb = value.as_bytes();
    unsafe {
        host_kv_set(
            kb.as_ptr() as u32, kb.len() as u32,
            vb.as_ptr() as u32, vb.len() as u32,
        )
    }
}

// ============================================================
// Data types — Plugin metadata
// ============================================================

#[derive(Serialize)]
struct Descriptor {
    r#type: &'static str,
    label: &'static str,
    short_label: &'static str,
    color: &'static str,
    version: &'static str,
    description: &'static str,
    config_fields: Vec<ConfigField>,
    view: View,
}

#[derive(Serialize)]
struct ConfigField {
    key: &'static str,
    label: &'static str,
    r#type: &'static str,
    required: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    placeholder: Option<&'static str>,
}

#[derive(Serialize)]
struct View {
    layout: &'static str,
    group_by: &'static str,
    searchable: bool,
    sortable: bool,
}

// ============================================================
// Data types — Refresh output
// ============================================================

#[derive(Serialize)]
struct RefreshResponse {
    streams: Vec<Stream>,
}

#[derive(Serialize)]
struct Stream {
    id: String,
    name: String,
    url: String,
    group: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    logo: Option<String>,
    tags: Vec<String>,
}

// ============================================================
// Data types — Windy API response
// ============================================================

#[derive(Deserialize)]
struct WindyResponse {
    #[serde(default)]
    webcams: Vec<Webcam>,
}

#[derive(Deserialize)]
struct Webcam {
    #[serde(alias = "webcamId")]
    webcam_id: u64,
    #[serde(default)]
    title: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    player: Option<PlayerInfo>,
    #[serde(default)]
    location: Option<LocationInfo>,
    #[serde(default)]
    images: Option<ImagesInfo>,
}

#[derive(Deserialize)]
struct PlayerInfo {
    #[serde(default)]
    day: Option<String>,
    #[serde(default)]
    lifetime: Option<String>,
}

#[derive(Deserialize)]
struct LocationInfo {
    #[serde(default)]
    city: String,
    #[serde(default)]
    country: String,
    #[serde(default, alias = "continentCode")]
    continent_code: String,
}

#[derive(Deserialize)]
struct ImagesInfo {
    #[serde(default)]
    current: Option<ImageUrls>,
}

#[derive(Deserialize)]
struct ImageUrls {
    #[serde(default)]
    preview: String,
    #[serde(default)]
    thumbnail: String,
}

// ============================================================
// Plugin exports
// ============================================================

#[no_mangle]
pub extern "C" fn describe() -> u64 {
    let desc = Descriptor {
        r#type: "webcams",
        label: "WindyCams",
        short_label: "WINDY",
        color: "#00bcd4",
        version: "1.0.0",
        description: "Live webcams from around the world via Windy.com",
        config_fields: vec![ConfigField {
            key: "windy_api_key",
            label: "Windy API Key",
            r#type: "password",
            required: true,
            placeholder: Some("Get free key at api.windy.com"),
        }],
        view: View {
            layout: "grouped_list",
            group_by: "group",
            searchable: true,
            sortable: true,
        },
    };
    return_json(&desc)
}

/// Fetches one page of webcams for a given continent.
fn fetch_webcams_page(api_key: &str, continent: &str, offset: u32) -> Option<WindyResponse> {
    let url = format!(
        "https://api.windy.com/webcams/api/v3/webcams?limit=50&offset={}&include=player,location,images&continent={}",
        offset, continent
    );
    let headers = format!("{{\"x-windy-api-key\":\"{}\"}}", api_key);

    let body = http_get_with_headers(&url, &headers)?;
    serde_json::from_slice::<WindyResponse>(&body).ok()
}

/// Fetches webcams for a continent, paginating up to max_pages (each page = 50).
fn fetch_continent(api_key: &str, continent: &str, max_pages: u32) -> Vec<Webcam> {
    let mut all = Vec::new();
    for page in 0..max_pages {
        let offset = page * 50;
        match fetch_webcams_page(api_key, continent, offset) {
            Some(resp) => {
                let count = resp.webcams.len();
                all.extend(resp.webcams);
                // If we got fewer than 50, no more pages.
                if count < 50 {
                    break;
                }
            }
            None => {
                log_warn(&format!(
                    "webcams: failed to fetch continent={} offset={}",
                    continent, offset
                ));
                break;
            }
        }
    }
    all
}

/// Convert a Webcam into a Stream, if it has a live player URL.
fn webcam_to_stream(cam: &Webcam) -> Option<Stream> {
    // Only include active webcams with a player URL.
    let player = cam.player.as_ref()?;
    if player.day.is_none() && player.lifetime.is_none() {
        return None;
    }

    let country = cam
        .location
        .as_ref()
        .map(|l| l.country.clone())
        .unwrap_or_default();
    let group = if country.is_empty() {
        "Other".to_string()
    } else {
        country
    };

    // Use the embed player URL as the stream URL.
    let url = format!(
        "https://webcams.windy.com/webcams/public/embed/player/{}/live",
        cam.webcam_id
    );

    let logo = cam.images.as_ref().and_then(|imgs| {
        imgs.current.as_ref().map(|c| {
            if !c.preview.is_empty() {
                c.preview.clone()
            } else {
                c.thumbnail.clone()
            }
        })
    });

    let mut tags = vec!["live".to_string()];
    if !cam.status.is_empty() {
        tags.push(cam.status.clone());
    }

    Some(Stream {
        id: cam.webcam_id.to_string(),
        name: cam.title.clone(),
        url,
        group,
        logo,
        tags,
    })
}

#[no_mangle]
pub extern "C" fn refresh(config_ptr: u32, config_len: u32) -> u64 {
    let config_data = read_input(config_ptr, config_len);
    let config: HashMap<String, serde_json::Value> =
        serde_json::from_slice(&config_data).unwrap_or_default();

    let api_key = config
        .get("windy_api_key")
        .and_then(|v| v.as_str())
        .unwrap_or("");

    if api_key.is_empty() {
        log_warn("webcams: no windy_api_key configured");
        return return_json(&RefreshResponse {
            streams: Vec::new(),
        });
    }

    log_info("webcams: refreshing webcam feeds from Windy.com");

    let continents = ["EU", "NA", "SA", "AS", "AF", "OC"];
    let max_pages: u32 = 4; // 4 pages x 50 = up to 200 per continent

    let mut seen: HashMap<u64, bool> = HashMap::new();
    let mut streams: Vec<Stream> = Vec::new();

    for continent in &continents {
        let cams = fetch_continent(api_key, continent, max_pages);
        log_info(&format!(
            "webcams: fetched {} webcams for continent {}",
            cams.len(),
            continent
        ));

        for cam in &cams {
            // Deduplicate by webcamId.
            if seen.contains_key(&cam.webcam_id) {
                continue;
            }
            seen.insert(cam.webcam_id, true);

            if let Some(stream) = webcam_to_stream(cam) {
                streams.push(stream);
            }
        }
    }

    log_info(&format!(
        "webcams: returning {} streams total",
        streams.len()
    ));
    return_json(&RefreshResponse { streams })
}

#[no_mangle]
pub extern "C" fn interact(action_ptr: u32, action_len: u32) -> u64 {
    let _ = read_input(action_ptr, action_len);
    return_json(&serde_json::json!({}))
}
