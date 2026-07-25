package media

type Chapter struct {
	Index     int
	StartTime float64
	EndTime   float64
	Title     string
}

type Title struct {
	Index    int
	Name     string
	Duration float64 // seconds; 0 if unknown
	IsMenu   bool
}

type StreamInfo struct {
	Index     int
	CodecName string
	Language  string
	Title     string
}
