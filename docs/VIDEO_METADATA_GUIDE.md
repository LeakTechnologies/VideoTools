# Video Metadata Guide for VideoTools

## Overview
This guide covers adding custom metadata fields to video files, NFO generation, and integration with VideoTools modules.

---

## 📦 Container Format Metadata Capabilities

### MP4 / MOV (MPEG-4)
**Metadata storage:** Atoms in `moov` container

**Standard iTunes-compatible tags:**
```
©nam - Title
©ART - Artist
©alb - Album
©day - Year
©gen - Genre
©cmt - Comment
desc - Description
©too - Encoding tool
©enc - Encoded by
cprt - Copyright
```

**Custom tags (with proper keys):**
```
----:com.apple.iTunes:DIRECTOR    - Director
----:com.apple.iTunes:PERFORMERS  - Performers
----:com.apple.iTunes:STUDIO      - Studio/Production
----:com.apple.iTunes:SERIES      - Series name
----:com.apple.iTunes:SCENE       - Scene number
----:com.apple.iTunes:CATEGORIES  - Categories/Tags
```

**Setting metadata with FFmpeg:**
```bash
ffmpeg -i input.mp4 -c copy \
  -metadata title="Scene Title" \
  -metadata artist="Performer Name" \
  -metadata album="Series Name" \
  -metadata date="2025" \
  -metadata genre="Category" \
  -metadata comment="Scene description" \
  -metadata description="Full scene info" \
  output.mp4
```

**Custom fields:**
```bash
ffmpeg -i input.mp4 -c copy \
  -metadata:s:v:0 custom_field="Custom Value" \
  output.mp4
```

---

### MKV (Matroska)
**Metadata storage:** Tags element (XML-based)

**Built-in tag support:**
```xml
<Tags>
  <Tag>
    <Simple>
      <Name>TITLE</Name>
      <String>Scene Title</String>
    </Simple>
    <Simple>
      <Name>ARTIST</Name>
      <String>Performer Name</String>
    </Simple>
    <Simple>
      <Name>DIRECTOR</Name>
      <String>Director Name</String>
    </Simple>
    <Simple>
      <Name>STUDIO</Name>
      <String>Production Studio</String>
    </Simple>
    <!-- Arbitrary custom tags -->
    <Simple>
      <Name>PERFORMERS</Name>
      <String>Performer 1, Performer 2</String>
    </Simple>
    <Simple>
      <Name>SCENE_NUMBER</Name>
      <String>EP042</String>
    </Simple>
    <Simple>
      <Name>CATEGORIES</Name>
      <String>Cat1, Cat2, Cat3</String>
    </Simple>
  </Tag>
</Tags>
```

**Setting metadata with FFmpeg:**
```bash
ffmpeg -i input.mkv -c copy \
  -metadata title="Scene Title" \
  -metadata artist="Performer Name" \
  -metadata director="Director" \
  -metadata studio="Studio Name" \
  output.mkv
```

**Advantages of MKV:**
- Unlimited custom tags (any key-value pairs)
- Can attach files (NFO, images, scripts)
- Hierarchical metadata structure
- Best for archival/preservation

---

### MOV (QuickTime)
Same as MP4 (both use MPEG-4 structure), but QuickTime supports additional proprietary tags.

---

## 📄 NFO File Format

NFO (Info) files are plain text/XML files that contain detailed metadata. Common in media libraries (Kodi, Plex, etc.).

### NFO Format for Movies:
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
  <title>Scene Title</title>
  <originaltitle>Original Title</originaltitle>
  <sorttitle>Sort Title</sorttitle>
  <year>2025</year>
  <releasedate>2025-12-04</releasedate>
  <plot>Scene description and plot summary</plot>
  <runtime>45</runtime> <!-- minutes -->
  <studio>Production Studio</studio>
  <director>Director Name</director>

  <actor>
    <name>Performer 1</name>
    <role>Role 1</role>
    <thumb>path/to/performer1.jpg</thumb>
  </actor>
  <actor>
    <name>Performer 2</name>
    <role>Role 2</role>
  </actor>

  <genre>Category 1</genre>
  <genre>Category 2</genre>

  <tag>Tag1</tag>
  <tag>Tag2</tag>

  <rating>8.5</rating>
  <userrating>9.0</userrating>

  <fileinfo>
    <streamdetails>
      <video>
        <codec>h264</codec>
        <width>1920</width>
        <height>1080</height>
        <durationinseconds>2700</durationinseconds>
        <aspect>1.777778</aspect>
      </video>
      <audio>
        <codec>aac</codec>
        <channels>2</channels>
      </audio>
    </streamdetails>
  </fileinfo>

  <!-- Custom fields -->
  <series>Series Name</series>
  <episode>42</episode>
  <scene_number>EP042</scene_number>
</movie>
```

### NFO Format for TV Episodes:
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<episodedetails>
  <title>Episode Title</title>
  <showtitle>Series Name</showtitle>
  <season>1</season>
  <episode>5</episode>
  <aired>2025-12-04</aired>
  <plot>Episode description</plot>
  <runtime>30</runtime>
  <director>Director Name</director>

  <actor>
    <name>Performer 1</name>
    <role>Character</role>
  </actor>

  <studio>Production Studio</studio>
  <rating>8.0</rating>
</episodedetails>
```

---

## 🛠️ VideoTools Integration Plan

### Module: **Metadata Editor** (New Module)
**Purpose:** Edit video metadata and generate NFO files

**Features:**
1. **Load video** → Extract existing metadata
2. **Edit fields** → Standard + custom fields
3. **NFO generation** → Auto-generate from metadata
4. **Embed metadata** → Write back to video file (lossless remux)
5. **Batch metadata** → Apply same metadata to multiple files
6. **Templates** → Save/load metadata templates

**UI Layout:**
```
┌─────────────────────────────────────────────────┐
│ < METADATA                                       │ ← Purple header
├─────────────────────────────────────────────────┤
│                                                  │
│  File: scene_042.mp4                            │
│                                                  │
│  ┌─ Basic Info ──────────────────────────────┐  │
│  │ Title:       [________________]           │  │
│  │ Studio:      [________________]           │  │
│  │ Series:      [________________]           │  │
│  │ Scene #:     [____]                       │  │
│  │ Date:        [2025-12-04]                 │  │
│  │ Duration:    45:23 (auto)                 │  │
│  └──────────────────────────────────────────────┘  │
│                                                  │
│  ┌─ Performers ────────────────────────────────┐  │
│  │ Performer 1: [________________] [X]        │  │
│  │ Performer 2: [________________] [X]        │  │
│  │                           [+ Add Performer] │  │
│  └──────────────────────────────────────────────┘  │
│                                                  │
│  ┌─ Categories/Tags ──────────────────────────┐  │
│  │ [Tag1] [Tag2] [Tag3]            [+ Add]    │  │
│  └──────────────────────────────────────────────┘  │
│                                                  │
│  ┌─ Description ────────────────────────────────┐  │
│  │ [Multiline text area for plot/description] │  │
│  │                                              │  │
│  └──────────────────────────────────────────────┘  │
│                                                  │
│  ┌─ Custom Fields ────────────────────────────┐  │
│  │ Director:  [________________]              │  │
│  │ IMDB ID:   [________________]              │  │
│  │ Custom 1:  [________________]              │  │
│  │                             [+ Add Field]   │  │
│  └──────────────────────────────────────────────┘  │
│                                                  │
│  [Generate NFO] [Embed in Video] [Save Template]│
│                                                  │
└─────────────────────────────────────────────────┘
```

---

## 🔧 Implementation Details

### 1. Reading Metadata
**Using FFprobe:**
```bash
ffprobe -v quiet -print_format json -show_format input.mp4

# Output includes:
{
  "format": {
    "filename": "input.mp4",
    "tags": {
      "title": "Scene Title",
      "artist": "Performer Name",
      "album": "Series Name",
      "date": "2025",
      "genre": "Category",
      "comment": "Description"
    }
  }
}
```

**Go implementation:**
```go
type VideoMetadata struct {
    Title       string
    Studio      string
    Series      string
    SceneNumber string
    Date        string
    Performers  []string
    Director    string
    Categories  []string
    Description string
    CustomFields map[string]string
}

func probeMetadata(path string) (*VideoMetadata, error) {
    cmd := exec.Command("ffprobe",
        "-v", "quiet",
        "-print_format", "json",
        "-show_format",
        path,
    )

    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var result struct {
        Format struct {
            Tags map[string]string `json:"tags"`
        } `json:"format"`
    }

    json.Unmarshal(output, &result)

    metadata := &VideoMetadata{
        Title:       result.Format.Tags["title"],
        Studio:      result.Format.Tags["studio"],
        Series:      result.Format.Tags["album"],
        Date:        result.Format.Tags["date"],
        Categories:  strings.Split(result.Format.Tags["genre"], ", "),
        Description: result.Format.Tags["comment"],
        CustomFields: make(map[string]string),
    }

    return metadata, nil
}
```

---

### 2. Writing Metadata
**Using FFmpeg (lossless remux):**
```go
func embedMetadata(inputPath string, metadata *VideoMetadata, outputPath string) error {
    args := []string{
        "-i", inputPath,
        "-c", "copy", // Lossless copy
    }

    // Add standard tags
    if metadata.Title != "" {
        args = append(args, "-metadata", fmt.Sprintf("title=%s", metadata.Title))
    }
    if metadata.Studio != "" {
        args = append(args, "-metadata", fmt.Sprintf("studio=%s", metadata.Studio))
    }
    if metadata.Series != "" {
        args = append(args, "-metadata", fmt.Sprintf("album=%s", metadata.Series))
    }
    if metadata.Date != "" {
        args = append(args, "-metadata", fmt.Sprintf("date=%s", metadata.Date))
    }
    if len(metadata.Categories) > 0 {
        args = append(args, "-metadata", fmt.Sprintf("genre=%s", strings.Join(metadata.Categories, ", ")))
    }
    if metadata.Description != "" {
        args = append(args, "-metadata", fmt.Sprintf("comment=%s", metadata.Description))
    }

    // Add custom fields
    for key, value := range metadata.CustomFields {
        args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, value))
    }

    args = append(args, outputPath)

    cmd := exec.Command("ffmpeg", args...)
    return cmd.Run()
}
```

---

### 3. Generating NFO
```go
func generateNFO(metadata *VideoMetadata, videoPath string) (string, error) {
    nfo := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
  <title>` + escapeXML(metadata.Title) + `</title>
  <studio>` + escapeXML(metadata.Studio) + `</studio>
  <series>` + escapeXML(metadata.Series) + `</series>
  <year>` + metadata.Date + `</year>
  <plot>` + escapeXML(metadata.Description) + `</plot>
`

    // Add performers
    for _, performer := range metadata.Performers {
        nfo += `  <actor>
    <name>` + escapeXML(performer) + `</name>
  </actor>
`
    }

    // Add categories/genres
    for _, category := range metadata.Categories {
        nfo += `  <genre>` + escapeXML(category) + `</genre>
`
    }

    // Add custom fields
    for key, value := range metadata.CustomFields {
        nfo += `  <` + key + `>` + escapeXML(value) + `</` + key + `>
`
    }

    nfo += `</movie>`

    // Save to file (same name as video + .nfo extension)
    nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
    return nfoPath, os.WriteFile(nfoPath, []byte(nfo), 0644)
}

func escapeXML(s string) string {
    s = strings.ReplaceAll(s, "&", "&amp;")
    s = strings.ReplaceAll(s, "<", "&lt;")
    s = strings.ReplaceAll(s, ">", "&gt;")
    s = strings.ReplaceAll(s, "\"", "&quot;")
    s = strings.ReplaceAll(s, "'", "&apos;")
    return s
}
```

---

### 4. Attaching NFO to MKV
MKV supports embedded attachments (like NFO files):

```bash
# Attach NFO file to MKV
mkvpropedit video.mkv --add-attachment scene_info.nfo --attachment-mime-type text/plain --attachment-name "scene_info.nfo"

# Or with FFmpeg (re-mux required)
ffmpeg -i input.mkv -i scene_info.nfo -c copy \
  -attach scene_info.nfo -metadata:s:t:0 mimetype=text/plain \
  output.mkv
```

**Go implementation:**
```go
func attachNFOtoMKV(mkvPath string, nfoPath string) error {
    cmd := exec.Command("mkvpropedit", mkvPath,
        "--add-attachment", nfoPath,
        "--attachment-mime-type", "text/plain",
        "--attachment-name", filepath.Base(nfoPath),
    )
    return cmd.Run()
}
```

---

## 📋 Metadata Templates

Allow users to save metadata templates for batch processing.

**Template JSON:**
```json
{
  "name": "Studio XYZ Default Template",
  "fields": {
    "studio": "Studio XYZ",
    "series": "Series Name",
    "categories": ["Category1", "Category2"],
    "custom_fields": {
      "director": "John Doe",
      "producer": "Jane Smith"
    }
  }
}
```

**Usage:**
1. User creates template with common studio/series info
2. Load template when editing new video
3. Only fill in unique fields (title, performers, date, scene #)
4. Batch apply template to multiple files

---

## 🎯 Use Cases

### 1. Adult Content Library
```
Title: "Scene Title"
Studio: "Production Studio"
Series: "Series Name - Season 2"
Scene Number: "EP042"
Performers: ["Performer A", "Performer B"]
Director: "Director Name"
Categories: ["Category1", "Category2", "Category3"]
Date: "2025-12-04"
Description: "Full scene description and plot"
```

### 2. Personal Video Archive
```
Title: "Birthday Party 2025"
Event: "John's 30th Birthday"
Location: "Los Angeles, CA"
People: ["John", "Sarah", "Mike", "Emily"]
Date: "2025-06-15"
Description: "John's surprise birthday party"
```

### 3. Movie Collection
```
Title: "Movie Title"
Original Title: "原題"
Director: "Christopher Nolan"
Year: "2024"
IMDB ID: "tt1234567"
Actors: ["Actor 1", "Actor 2"]
Genre: ["Sci-Fi", "Thriller"]
Rating: "8.5/10"
```

---

## 🔌 Integration with Existing Modules

### Convert Module
- **Checkbox**: "Preserve metadata" (default: on)
- **Checkbox**: "Copy metadata from source" (default: on)
- Allow adding/editing metadata before conversion

### Inspect Module
- **Add tab**: "Metadata" to view/edit metadata
- Show both standard and custom fields
- Quick edit without re-encoding

### Compare Module
- **Add**: "Compare Metadata" button
- Show metadata diff between two files
- Highlight differences

---

## 🚀 Implementation Roadmap

### Phase 1: Basic Metadata Support (Week 1)
- Read metadata with ffprobe
- Display in Inspect module
- Edit basic fields (title, artist, date, comment)
- Write metadata with FFmpeg (lossless)

### Phase 2: NFO Generation (Week 2)
- NFO file generation
- Save alongside video file
- Load NFO and populate fields
- Template system

### Phase 3: Advanced Metadata (Week 3)
- Custom fields support
- Performers list
- Categories/tags
- Metadata Editor module UI

### Phase 4: Batch & Templates (Week 4)
- Metadata templates
- Batch apply to multiple files
- MKV attachment support (embed NFO)

---

## 📚 References

### FFmpeg Metadata Documentation
- https://ffmpeg.org/ffmpeg-formats.html#Metadata
- https://wiki.multimedia.cx/index.php/FFmpeg_Metadata

### NFO Format Standards
- Kodi NFO: https://kodi.wiki/view/NFO_files
- Plex Agents: https://support.plex.tv/articles/

### Matroska Tags
- https://www.matroska.org/technical/specs/tagging/index.html

---

## ✅ Summary

**Yes, you can absolutely store custom metadata in video files!**

**Best format for rich metadata:** MKV (unlimited custom tags + file attachments)

**Most compatible:** MP4/MOV (iTunes tags work in QuickTime, VLC, etc.)

**Recommended approach for VideoTools:**
1. Support both embedded metadata (in video file) AND sidecar NFO files
2. MKV: Embed NFO as attachment + metadata tags
3. MP4: Metadata tags + separate .nfo file
4. Allow users to choose what metadata to embed
5. Generate NFO for media center compatibility (Kodi, Plex, Jellyfin)

**Next steps:**
1. Add basic metadata reading to `probeVideo()` function
2. Add metadata display to Inspect module
3. Create Metadata Editor module
4. Implement NFO generation
5. Add metadata templates

This would be a killer feature for VideoTools! 🚀
