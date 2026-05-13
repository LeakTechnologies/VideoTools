package media

type Chapter struct {
	Index     int
	StartTime float64
	EndTime   float64
	Title     string
}

type StreamInfo struct {
	Index     int
	CodecName string
	Language  string
	Title     string
}
