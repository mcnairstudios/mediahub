//! MediaHub Demo source plugin.
//!
//! Exports the WASM ABI expected by the MediaHub host:
//!   - `alloc(size) -> ptr`        allocate guest memory for host writes
//!   - `describe() -> packed`      return plugin descriptor JSON
//!   - `refresh(ptr, len) -> packed` return streams JSON
//!
//! Return values pack (ptr, len) as `(ptr as u64) << 32 | len as u64`.

use serde::Serialize;

// ---------------------------------------------------------------------------
// Host imports (provided by the MediaHub WASM host via the "env" module)
// ---------------------------------------------------------------------------

extern "C" {
    fn host_log(level: u32, msg_ptr: u32, msg_len: u32);
}

/// Log a message to the host at INFO level.
fn log_info(msg: &str) {
    unsafe {
        host_log(1, msg.as_ptr() as u32, msg.len() as u32);
    }
}

// ---------------------------------------------------------------------------
// Memory allocation for host → guest data transfer
// ---------------------------------------------------------------------------

/// Allocate `size` bytes of guest memory and return the pointer.
/// The host calls this before writing data into guest memory.
#[no_mangle]
pub extern "C" fn alloc(size: u32) -> u32 {
    if size == 0 {
        return 0;
    }
    let layout = std::alloc::Layout::from_size_align(size as usize, 1).unwrap();
    let ptr = unsafe { std::alloc::alloc(layout) };
    if ptr.is_null() {
        return 0;
    }
    ptr as u32
}

/// Pack a (ptr, len) pair into a single u64 for return to the host.
/// High 32 bits = pointer, low 32 bits = length.
fn pack_ptr_len(ptr: u32, len: u32) -> u64 {
    ((ptr as u64) << 32) | (len as u64)
}

/// Serialize a value to JSON bytes, write them into freshly allocated guest
/// memory, and return the packed (ptr, len).
fn return_json<T: Serialize>(value: &T) -> u64 {
    let json = match serde_json::to_vec(value) {
        Ok(v) => v,
        Err(_) => return 0,
    };
    let len = json.len() as u32;
    let ptr = self::alloc(len);
    if ptr == 0 {
        return 0;
    }
    unsafe {
        std::ptr::copy_nonoverlapping(json.as_ptr(), ptr as *mut u8, json.len());
    }
    pack_ptr_len(ptr, len)
}

// ---------------------------------------------------------------------------
// Data types matching the host JSON contract
// ---------------------------------------------------------------------------

#[derive(Serialize)]
struct PluginDescriptor {
    r#type: &'static str,
    label: &'static str,
    short_label: &'static str,
    color: &'static str,
    version: &'static str,
    description: &'static str,
    config_fields: Vec<ConfigField>,
}

#[derive(Serialize)]
struct ConfigField {
    key: &'static str,
    label: &'static str,
    r#type: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    required: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    default: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    placeholder: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    help_text: Option<&'static str>,
}

#[derive(Serialize)]
struct RefreshResult {
    streams: Vec<PluginStream>,
}

#[derive(Serialize)]
struct PluginStream {
    name: &'static str,
    url: &'static str,
    group: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    vod_type: Option<&'static str>,
}

// ---------------------------------------------------------------------------
// Demo stream definitions (same as the Go demo plugin)
// ---------------------------------------------------------------------------

const DEMO_STREAMS: &[PluginStream] = &[
    PluginStream {
        name: "Big Buck Bunny",
        url: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
        group: "Demo - Movies",
        vod_type: Some("movie"),
    },
    PluginStream {
        name: "Sintel",
        url: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/Sintel.mp4",
        group: "Demo - Movies",
        vod_type: Some("movie"),
    },
    PluginStream {
        name: "Tears of Steel",
        url: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/TearsOfSteel.mp4",
        group: "Demo - Movies",
        vod_type: Some("movie"),
    },
    PluginStream {
        name: "Elephant's Dream",
        url: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4",
        group: "Demo - Movies",
        vod_type: Some("movie"),
    },
    PluginStream {
        name: "NASA Live",
        url: "https://ntv1.akamaized.net/hls/live/2014075/NASA-NTV1-HLS/master.m3u8",
        group: "Demo - Live",
        vod_type: None,
    },
    PluginStream {
        name: "Bloomberg TV",
        url: "https://www.bloomberg.com/media-manifest/streams/us.m3u8",
        group: "Demo - Live",
        vod_type: None,
    },
];

// ---------------------------------------------------------------------------
// Exported WASM functions
// ---------------------------------------------------------------------------

/// Called by the host to get plugin metadata.
/// Returns packed (ptr, len) pointing to JSON descriptor.
#[no_mangle]
pub extern "C" fn describe() -> u64 {
    let desc = PluginDescriptor {
        r#type: "demo",
        label: "Demo Streams",
        short_label: "DEMO",
        color: "#607d8b",
        version: "1.0.0",
        description: "Curated public test streams for demonstration and testing",
        config_fields: vec![],
    };
    return_json(&desc)
}

/// Called by the host to refresh streams.
/// `_config_ptr` and `_config_len` point to config JSON (unused for Demo).
/// Returns packed (ptr, len) pointing to JSON refresh result.
#[no_mangle]
pub extern "C" fn refresh(_config_ptr: u32, _config_len: u32) -> u64 {
    log_info("refreshing demo streams");

    let result = RefreshResult {
        streams: DEMO_STREAMS
            .iter()
            .map(|s| PluginStream {
                name: s.name,
                url: s.url,
                group: s.group,
                vod_type: s.vod_type,
            })
            .collect(),
    };

    return_json(&result)
}
