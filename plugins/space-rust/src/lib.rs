use serde::Serialize;
use serde_json::Value;
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

fn log_error(msg: &str) {
    let bytes = msg.as_bytes();
    unsafe { host_log(3, bytes.as_ptr() as u32, bytes.len() as u32) }
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
// Data types -- Plugin metadata
// ============================================================

#[derive(Serialize)]
struct Descriptor {
    r#type: &'static str,
    label: &'static str,
    short_label: &'static str,
    color: &'static str,
    version: &'static str,
    description: &'static str,
    config_fields: Vec<Value>,
    view: View,
    interactions: Vec<Value>,
}

#[derive(Serialize)]
struct View {
    layout: &'static str,
    group_by: &'static str,
    searchable: bool,
    sortable: bool,
}

// ============================================================
// Data types -- Refresh output
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
    vod_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    year: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    tags: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    episode_name: Option<String>,
}

// ============================================================
// API response types -- use Value for flexible fields
// ============================================================

fn get_str(v: &Value, key: &str) -> String {
    v.get(key).and_then(|x| x.as_str()).unwrap_or("").to_string()
}

fn get_opt_str(v: &Value, key: &str) -> Option<String> {
    v.get(key).and_then(|x| x.as_str()).map(|s| s.to_string())
}

/// Extract status abbreviation -- can be {"abbrev":"Success"} or "Success" or null
fn extract_status_abbrev(v: &Value) -> String {
    match v {
        Value::Object(obj) => {
            obj.get("abbrev").and_then(|a| a.as_str()).unwrap_or("").to_string()
        }
        Value::String(s) => s.clone(),
        _ => String::new(),
    }
}

/// Extract LSP name -- can be {"name":"SpaceX"} or "SpaceX" or null; fallback to lsp_name
fn extract_lsp_name(lsp: &Value, lsp_name_flat: &str) -> String {
    if !lsp_name_flat.is_empty() {
        return lsp_name_flat.to_string();
    }
    match lsp {
        Value::Object(obj) => {
            obj.get("name").and_then(|n| n.as_str()).unwrap_or("Unknown").to_string()
        }
        Value::String(s) => s.clone(),
        _ => "Unknown".to_string(),
    }
}

/// Extract mission description -- can be {"description":"..."} or "mission name" or null
fn extract_mission_description(v: &Value) -> String {
    match v {
        Value::Object(obj) => {
            obj.get("description").and_then(|d| d.as_str()).unwrap_or("").to_string()
        }
        Value::String(s) => s.clone(),
        _ => String::new(),
    }
}

/// Find best video URL (lowest priority number)
fn best_video_url(vid_urls: &Value) -> String {
    let arr = match vid_urls.as_array() {
        Some(a) if !a.is_empty() => a,
        _ => return String::new(),
    };

    let mut best_url = "";
    let mut best_priority = i64::MAX;

    for entry in arr {
        let url = entry.get("url").and_then(|u| u.as_str()).unwrap_or("");
        let priority = entry.get("priority").and_then(|p| p.as_i64()).unwrap_or(i64::MAX);
        if priority < best_priority {
            best_priority = priority;
            best_url = url;
        }
    }

    best_url.to_string()
}

/// Parse date string (RFC3339) into ("Jan 02, 2006", "2006")
fn format_date(net: &str) -> (String, String) {
    // Minimal date parser for "2025-01-14T06:30:00Z" style dates
    // We need: month name, day, year
    if net.len() < 10 {
        return (net.to_string(), String::new());
    }

    let year_str = &net[0..4];
    let month_str = &net[5..7];
    let day_str = &net[8..10];

    let month_name = match month_str {
        "01" => "Jan",
        "02" => "Feb",
        "03" => "Mar",
        "04" => "Apr",
        "05" => "May",
        "06" => "Jun",
        "07" => "Jul",
        "08" => "Aug",
        "09" => "Sep",
        "10" => "Oct",
        "11" => "Nov",
        "12" => "Dec",
        _ => return (net.to_string(), String::new()),
    };

    // Parse day as number to format without leading zero... actually Go's "02" format
    // keeps leading zero. Let's match exactly: "Jan 02, 2006"
    let formatted = format!("{} {}, {}", month_name, day_str, year_str);
    (formatted, year_str.to_string())
}

fn launch_to_stream(launch: &Value) -> Stream {
    let id = get_str(launch, "id");
    let raw_name = get_str(launch, "name");
    let net = get_str(launch, "net");
    let image = get_opt_str(launch, "image");

    let status = launch.get("status").unwrap_or(&Value::Null);
    let lsp = launch.get("launch_service_provider").unwrap_or(&Value::Null);
    let mission = launch.get("mission").unwrap_or(&Value::Null);
    let lsp_name_flat = get_str(launch, "lsp_name");
    let vid_urls = launch.get("vidURLs").unwrap_or(&Value::Null);

    let (date_formatted, year) = format_date(&net);
    let status_abbrev = extract_status_abbrev(status);
    let lsp_name = extract_lsp_name(lsp, &lsp_name_flat);
    let mission_desc = extract_mission_description(mission);
    let video_url = best_video_url(vid_urls);

    // Format name with date, matching Go: "Name (Jan 02, 2006)"
    let name = if date_formatted != net && !date_formatted.is_empty() {
        format!("{} ({})", raw_name, date_formatted)
    } else {
        raw_name
    };

    let tags = if !status_abbrev.is_empty() {
        Some(vec![status_abbrev.to_lowercase()])
    } else {
        None
    };

    let episode_name = if mission_desc.is_empty() {
        None
    } else {
        Some(mission_desc)
    };

    let year_opt = if year.is_empty() { None } else { Some(year) };

    // logo: match Go behavior -- empty string becomes None via skip_serializing_if
    let logo = match &image {
        Some(s) if !s.is_empty() => Some(s.clone()),
        _ => None,
    };

    Stream {
        id,
        name,
        url: video_url,
        group: lsp_name,
        logo,
        vod_type: "movie".to_string(),
        year: year_opt,
        tags,
        episode_name,
    }
}

/// Fetch paginated results from the given URL, up to max_pages.
fn fetch_pages(start_url: &str, max_pages: usize) -> Vec<Value> {
    let mut all: Vec<Value> = Vec::new();
    let mut url = start_url.to_string();

    for page in 0..max_pages {
        if url.is_empty() {
            break;
        }

        log_info(&format!("fetching page {}: {}", page + 1, url));

        let body = match http_get_with_headers(&url, "{}") {
            Some(b) => b,
            None => {
                log_error(&format!("http error on page {}", page + 1));
                break;
            }
        };

        let resp: Value = match serde_json::from_slice(&body) {
            Ok(v) => v,
            Err(_) => {
                log_error(&format!("json parse error on page {}", page + 1));
                break;
            }
        };

        if let Some(results) = resp.get("results").and_then(|r| r.as_array()) {
            all.extend(results.iter().cloned());
        } else {
            log_error(&format!("results parse error on page {}", page + 1));
            break;
        }

        // Follow pagination
        match resp.get("next") {
            Some(Value::String(next)) if !next.is_empty() => {
                url = next.clone();
            }
            _ => break,
        }
    }

    all
}

// ============================================================
// Plugin exports
// ============================================================

#[no_mangle]
pub extern "C" fn describe() -> u64 {
    let desc = Descriptor {
        r#type: "spacex",
        label: "Space Launches",
        short_label: "SPACE",
        color: "#1e88e5",
        version: "1.0.0",
        description: "Space launch streams from the Launch Library 2 API (thespacedevs.com)",
        config_fields: vec![],
        view: View {
            layout: "grouped_list",
            group_by: "group",
            searchable: true,
            sortable: true,
        },
        interactions: vec![],
    };
    return_json(&desc)
}

#[no_mangle]
pub extern "C" fn refresh(config_ptr: u32, config_len: u32) -> u64 {
    let _ = read_input(config_ptr, config_len);

    let mut streams: Vec<Stream> = Vec::new();

    // Fetch past launches (detailed mode, up to 10 pages)
    let past_launches = fetch_pages(
        "https://ll.thespacedevs.com/2.2.0/launch/previous/?mode=detailed&limit=50",
        10,
    );
    for l in &past_launches {
        streams.push(launch_to_stream(l));
    }
    log_info(&format!("fetched {} past launches", past_launches.len()));

    // Fetch upcoming launches (list mode, up to 5 pages)
    let upcoming_launches = fetch_pages(
        "https://ll.thespacedevs.com/2.2.0/launch/upcoming/?mode=list&limit=50",
        5,
    );
    for l in &upcoming_launches {
        streams.push(launch_to_stream(l));
    }
    log_info(&format!("fetched {} upcoming launches", upcoming_launches.len()));

    log_info(&format!("total streams: {}", streams.len()));

    return_json(&RefreshResponse { streams })
}

#[no_mangle]
pub extern "C" fn interact(action_ptr: u32, action_len: u32) -> u64 {
    let _ = read_input(action_ptr, action_len);
    let data = b"{}";
    let ptr = data.as_ptr() as u32;
    let len = data.len() as u32;
    pack_ptr_len(ptr, len)
}
