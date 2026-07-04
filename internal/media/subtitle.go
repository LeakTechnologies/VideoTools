//go:build native_media

package media

/*
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
#include <libavutil/dict.h>
#include <libavutil/opt.h>
#include <libavutil/time.h>
*/
import "C"
import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

type SubtitleType int

const (
	SubtitleTypeUnknown SubtitleType = iota
	SubtitleTypeText
	SubtitleTypeASS
	SubtitleTypeSRT
)

type Subtitle struct {
	Index     int
	StartTime time.Duration
	EndTime   time.Duration
	Text      string
	ASSStyle  string
	Format    SubtitleType
}

type SubtitleTrack struct {
	Index     int
	Language  string
	CodecName string
	Title     string
	IsForced  bool
	IsDefault bool
}

type SubtitleExtractor struct {
	formatCtx *C.AVFormatContext
	tracks    []SubtitleTrack
	mu        sync.Mutex
	streamIdx int
	timeBase  float64
}

func NewSubtitleExtractor() *SubtitleExtractor {
	return &SubtitleExtractor{
		tracks: make([]SubtitleTrack, 0),
	}
}

func (se *SubtitleExtractor) Open(path string) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	logging.Info(logging.CatPlayer, "Opening subtitle extractor for: %s", path)

	if C.avformat_open_input(&se.formatCtx, cPath, nil, nil) != 0 {
		return fmt.Errorf("failed to open input file: %s", path)
	}

	if C.avformat_find_stream_info(se.formatCtx, nil) < 0 {
		C.avformat_close_input(&se.formatCtx)
		return fmt.Errorf("failed to find stream info")
	}

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(se.formatCtx.streams))

	for i := 0; i < int(se.formatCtx.nb_streams); i++ {
		stream := streams[i]
		if stream == nil {
			continue
		}

		if stream.codecpar.codec_type != C.AVMEDIA_TYPE_SUBTITLE {
			continue
		}

		codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
		if codec == nil {
			logging.Warning(logging.CatPlayer, "No decoder for subtitle stream %d", i)
			continue
		}

		codecName := C.GoString((*C.char)(unsafe.Pointer(codec.name)))

		var language, title string
		if stream.metadata != nil {
			var entry *C.AVDictionaryEntry
			for {
				entry = C.av_dict_iterate(stream.metadata, entry)
				if entry == nil {
					break
				}
				key := C.GoString(entry.key)
				if key == "language" {
					language = C.GoString(entry.value)
				} else if key == "title" {
					title = C.GoString(entry.value)
				}
			}
		}

		isForced := false
		isDefault := false
		if stream.disposition != 0 {
			isForced = (stream.disposition & C.AV_DISPOSITION_FORCED) != 0
			isDefault = (stream.disposition & C.AV_DISPOSITION_DEFAULT) != 0
		}

		track := SubtitleTrack{
			Index:     i,
			Language:  language,
			CodecName: codecName,
			Title:     title,
			IsForced:  isForced,
			IsDefault: isDefault,
		}
		se.tracks = append(se.tracks, track)
	}

	if len(se.tracks) == 0 {
		C.avformat_close_input(&se.formatCtx)
		return fmt.Errorf("no subtitle streams found")
	}

	se.streamIdx = se.tracks[0].Index
	se.timeBase = float64(streams[se.streamIdx].time_base.num) / float64(streams[se.streamIdx].time_base.den)

	logging.Info(logging.CatPlayer, "Found %d subtitle tracks", len(se.tracks))
	return nil
}

func (se *SubtitleExtractor) GetTracks() []SubtitleTrack {
	se.mu.Lock()
	defer se.mu.Unlock()
	result := make([]SubtitleTrack, len(se.tracks))
	copy(result, se.tracks)
	return result
}

func (se *SubtitleExtractor) SelectTrack(index int) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	if index < 0 || index >= len(se.tracks) {
		return fmt.Errorf("invalid track index: %d", index)
	}

	se.streamIdx = se.tracks[index].Index

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(se.formatCtx.streams))
	se.timeBase = float64(streams[se.streamIdx].time_base.num) / float64(streams[se.streamIdx].time_base.den)

	return nil
}

func (se *SubtitleExtractor) ExtractSubtitles() ([]Subtitle, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.formatCtx == nil {
		return nil, fmt.Errorf("no file opened")
	}

	var subtitles []Subtitle

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(se.formatCtx.streams))
	stream := streams[se.streamIdx]
	codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
	if codec == nil {
		return nil, fmt.Errorf("no decoder found")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		return nil, fmt.Errorf("failed to allocate codec context")
	}
	defer C.avcodec_free_context(&codecCtx)

	C.avcodec_parameters_to_context(codecCtx, stream.codecpar)
	if C.avcodec_open2(codecCtx, codec, nil) < 0 {
		return nil, fmt.Errorf("failed to open codec")
	}

	frame := C.av_frame_alloc()
	if frame == nil {
		return nil, fmt.Errorf("failed to allocate frame")
	}
	defer C.av_frame_free(&frame)

	pkt := C.av_packet_alloc()
	if pkt == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	defer C.av_packet_free(&pkt)

	subIdx := 0
	for {
		if C.av_read_frame(se.formatCtx, pkt) < 0 {
			break
		}
		defer C.av_packet_unref(pkt)

		if int(pkt.stream_index) != se.streamIdx {
			continue
		}

		if C.avcodec_send_packet(codecCtx, pkt) < 0 {
			logging.Warning(logging.CatPlayer, "Failed to send subtitle packet")
			continue
		}

		for C.avcodec_receive_frame(codecCtx, frame) >= 0 {
			sub := se.decodeSubtitleFrame(frame, subIdx)
			if sub != nil {
				subtitles = append(subtitles, *sub)
				subIdx++
			}
		}
	}

	return subtitles, nil
}

func (se *SubtitleExtractor) ExtractSubtitlesToTime(endTime time.Duration) ([]Subtitle, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.formatCtx == nil {
		return nil, fmt.Errorf("no file opened")
	}

	var subtitles []Subtitle

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(se.formatCtx.streams))
	stream := streams[se.streamIdx]
	codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
	if codec == nil {
		return nil, fmt.Errorf("no decoder found")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		return nil, fmt.Errorf("failed to allocate codec context")
	}
	defer C.avcodec_free_context(&codecCtx)

	C.avcodec_parameters_to_context(codecCtx, stream.codecpar)
	if C.avcodec_open2(codecCtx, codec, nil) < 0 {
		return nil, fmt.Errorf("failed to open codec")
	}

	frame := C.av_frame_alloc()
	if frame == nil {
		return nil, fmt.Errorf("failed to allocate frame")
	}
	defer C.av_frame_free(&frame)

	pkt := C.av_packet_alloc()
	if pkt == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	defer C.av_packet_free(&pkt)

	subIdx := 0
	for {
		if C.av_read_frame(se.formatCtx, pkt) < 0 {
			break
		}
		defer C.av_packet_unref(pkt)

		if int(pkt.stream_index) != se.streamIdx {
			continue
		}

		subPts := float64(pkt.pts) * se.timeBase
		if subPts*float64(time.Second) > endTime.Seconds() {
			C.av_packet_unref(pkt)
			break
		}

		if C.avcodec_send_packet(codecCtx, pkt) < 0 {
			continue
		}

		for C.avcodec_receive_frame(codecCtx, frame) >= 0 {
			sub := se.decodeSubtitleFrame(frame, subIdx)
			if sub != nil && sub.EndTime <= endTime {
				subtitles = append(subtitles, *sub)
				subIdx++
			}
		}
	}

	return subtitles, nil
}

func (se *SubtitleExtractor) decodeSubtitleFrame(frame *C.AVFrame, idx int) *Subtitle {
	if frame == nil {
		return nil
	}

	var startTime, endTime time.Duration
	format := SubtitleTypeUnknown

	if frame.pts != C.AV_NOPTS_VALUE {
		startTime = time.Duration(float64(frame.pts) * se.timeBase * float64(time.Second))
	}

	if frame.duration > 0 {
		endTime = time.Duration(float64(frame.duration)*se.timeBase*float64(time.Second)) + startTime
	}

	text := ""

	if frame.buf[0] != nil && frame.buf[0].data != nil {
		text = C.GoString((*C.char)(unsafe.Pointer(frame.buf[0].data)))
		if strings.Contains(text, "Dialogue:") {
			format = SubtitleTypeASS
		} else {
			format = SubtitleTypeText
		}
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	return &Subtitle{
		Index:     idx,
		StartTime: startTime,
		EndTime:   endTime,
		Text:      text,
		Format:    format,
	}
}

func (se *SubtitleExtractor) ExportToSRT(subtitles []Subtitle, path string) error {
	var sb strings.Builder

	for i, sub := range subtitles {
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n",
			formatSRTTime(sub.StartTime),
			formatSRTTime(sub.EndTime)))
		sb.WriteString(sub.Text)
		sb.WriteString("\n\n")
	}

	return writeFile(path, []byte(sb.String()))
}

func (se *SubtitleExtractor) ExportToASS(subtitles []Subtitle, path string) error {
	var sb strings.Builder

	sb.WriteString("[Script Info]\n")
	sb.WriteString("Title: Generated by VideoTools\n")
	sb.WriteString("ScriptType: v4.00+\n")
	sb.WriteString("Collisions: Normal\n")
	sb.WriteString("PlayDepth: 0\n\n")

	sb.WriteString("[V4+ Styles]\n")
	sb.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	sb.WriteString("Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n\n")

	sb.WriteString("[Events]\n")
	sb.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	for _, sub := range subtitles {
		sb.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
			formatASSTime(sub.StartTime),
			formatASSTime(sub.EndTime),
			escapeASSText(sub.Text)))
	}

	return writeFile(path, []byte(sb.String()))
}

func formatSRTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func formatASSTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	centis := (int(d.Milliseconds()) % 1000) / 10
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centis)
}

func escapeASSText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "{", "\\{")
	text = strings.ReplaceAll(text, "\n", "\\N")
	return text
}

func (se *SubtitleExtractor) Close() {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.formatCtx != nil {
		C.avformat_close_input(&se.formatCtx)
		se.formatCtx = nil
	}
}

func writeFile(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (se *SubtitleExtractor) HasSubtitles() bool {
	se.mu.Lock()
	defer se.mu.Unlock()
	return len(se.tracks) > 0
}

func (se *SubtitleExtractor) TrackCount() int {
	se.mu.Lock()
	defer se.mu.Unlock()
	return len(se.tracks)
}
