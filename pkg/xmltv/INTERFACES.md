# xmltv interfaces

## Types

### Channel
| Field       | Type   | Description              |
|-------------|--------|--------------------------|
| ID          | string | XMLTV channel identifier |
| DisplayName | string | Human-readable name      |
| Icon        | string | Icon/logo URL            |

### Programme
| Field       | Type     | Description                                     |
|-------------|----------|-------------------------------------------------|
| ChannelID   | string   | References Channel.ID                           |
| Title       | string   | Programme title                                 |
| Subtitle    | string   | Episode subtitle or empty                       |
| Description | string   | Programme description or empty                  |
| Start       | time.Time| Start time with timezone                        |
| Stop        | time.Time| Stop time with timezone                         |
| Categories  | []string | Genre/category tags (empty slice if none)       |
| Rating      | string   | Content rating value or empty                   |
| EpisodeNum  | string   | Episode number string or empty                  |
| IsNew       | bool     | True if has episode-num and no previously-shown  |
| Credits     | Credits  | Directors and actors                            |

### Credits
| Field     | Type     | Description                        |
|-----------|----------|------------------------------------|
| Directors | []string | Director names (empty slice if none)|
| Actors    | []string | Actor names (empty slice if none)  |

## Functions

### Parse(r io.Reader) ([]Channel, []Programme, error)

Reads XMLTV XML from `r` and returns parsed channels and programmes. Empty input returns empty slices with no error. All slice fields are initialized (never nil).

## Dependencies

- No external dependencies (stdlib only)
- Does not import any other mediahub packages
- Caller is responsible for converting to domain types (e.g., epg.Program)
