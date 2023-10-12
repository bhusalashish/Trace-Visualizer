package tracevisualizer

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	constructorKey = "constructor"
	destructorKey  = "destructor"
)

func isConstructor(line string) bool {
	return strings.HasSuffix(line, constructorKey)
}
func isDestructor(line string) bool {
	return strings.HasSuffix(line, destructorKey)
}

func parseLogFile(filename string) (lines []*parsedLine, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	// i := 10
	for scanner.Scan() {
		line := scanner.Text()
		parser := newLineParse()
		parsed := parser.parseLine(line)
		lines = append(lines, parsed)
	}

	err = scanner.Err()
	if err != nil {
		return
	}

	return
}

func Parse(filename string, regex string) {
	parsed, err := parseLogFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	body, err := constructTraceBody(parsed)
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.Marshal(TraceReq{Data: []*TraceReqBody{body}})
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.Write(b)
	// fmt.Printf("%+v\n", body)
}

type TraceReq struct {
	Data   []*TraceReqBody `json:"data"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
	Errors []string        `json:"errors"`
}

type TraceReqBody struct {
	TraceID   string                 `json:"traceID"`
	Spans     []*Span                `json:"spans"`
	Processes map[string]interface{} `json:"processes"`
	Warnings  []string               `json:"warnings"`
}

type SpanReferences struct {
	RefType string `json:"refType"`
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}
type SpanTags struct {
	Key     string `json:"key"`
	TagType string `json:"type"`
	Value   string `json:"value"`
}

type SpanLog struct {
	Timestamp int64          `json:"timestamp"`
	Fields    []SpanLogField `json:"fields"`
}

type SpanLogField struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Span struct {
	TraceID       string           `json:"traceID"`
	SpanID        string           `json:"spanID"`
	OperationName string           `json:"operationName"`
	References    []SpanReferences `json:"references"`
	StartTime     int64            `json:"startTime"`
	Duration      int64            `json:"duration"`
	Tags          []SpanTags       `json:"tags"`
	ProcessID     string           `json:"processID"`
	Warnings      []string         `json:"warnings"`
	SpanLogs      []SpanLog        `json:"logs"`
}

var id = 1

func getUniqueID() string {
	bs := []byte(fmt.Sprintf("%8d", id))
	id++
	return hex.EncodeToString(bs[:])
}

func constructTraceBody(parsedLines []*parsedLine) (*TraceReqBody, error) {
	stack := &Stack{}
	rootSpan := &parsedLine{funcName: "root", spanID: getUniqueID(), timestamp: parsedLines[0].timestamp}
	stack.Push(rootSpan)

	// Insert root span
	spans := []*Span{&Span{
		OperationName: rootSpan.funcName,
		StartTime:     rootSpan.timestamp,
		TraceID:       rootSpan.spanID,
		SpanID:        rootSpan.spanID,
		ProcessID:     "p1",
	}}

	processes := map[string]interface{}{}

	processIDMap := make(map[string]bool)
	currPID := 1

	for _, line := range parsedLines {
		switch line.typeOfLine {
		case constructor:
			stack.Push(line)
			line.spanID = getUniqueID()
		case destructor:
			constructorLine, err := stack.Pop()
			if err != nil {
				return nil, err
			}

			spanLogs := []SpanLog{}
			for _, childLine := range constructorLine.directChildParsedLines {
				spanLog := SpanLog{
					Timestamp: childLine.timestamp,
					Fields: []SpanLogField{
						SpanLogField{
							Key:   fmt.Sprintf("TRACE%s", childLine.traceLevel),
							Type:  "string",
							Value: strings.Join(childLine.infoLogs, " "),
						},
					},
				}
				spanLogs = append(spanLogs, spanLog)
			}
			// fmt.Printf("%+v\n", spanLogs)

			span := &Span{
				Duration:      line.timestamp - constructorLine.timestamp,
				OperationName: line.funcName,
				StartTime:     constructorLine.timestamp,
				TraceID:       rootSpan.spanID,
				SpanID:        constructorLine.spanID,
				ProcessID:     fmt.Sprintf("p%d", currPID),
				References: []SpanReferences{
					SpanReferences{
						RefType: "CHILD_OF",
						SpanID:  stack.Top().spanID,
						TraceID: rootSpan.spanID,
					},
				},
				SpanLogs: spanLogs,
			}
			spans = append(spans, span)

			if _, ok := processIDMap[constructorLine.traceHandle]; !ok {
				currPID = len(processes) + 1
				processes[fmt.Sprintf("p%d", len(processes)+1)] = map[string]interface{}{
					"serviceName": constructorLine.traceHandle,
				}
				processIDMap[constructorLine.traceHandle] = true
			}
			// fmt.Println(line.funcName, line.timestamp-constructorLine.timestamp)

		case info:
			// stack.Top().infoLogs = append(stack.Top().infoLogs, line.infoLogs...)
			stack.Top().directChildParsedLines = append(stack.Top().directChildParsedLines, line)
			// fmt.Printf("%+v\n", stack.Top())
		}
	}

	// We need to set the duration of the root span
	spans[0].Duration = spans[1].Duration

	body := &TraceReqBody{
		TraceID:   rootSpan.spanID,
		Spans:     spans,
		Processes: processes,
		// Processes: map[string]interface{}{
		// 	"p1": map[string]interface{}{
		// 		"serviceName": parsedLines[0].traceHandle,
		// 	},
		// },
	}
	return body, nil
}

//	"tags": []map[string]string{
//		{
//			"key":   "client-uuid",
//			"type":  "string",
//			"value": "8ce59807df100f4",
//		},
//		{
//			"key":   "host.name",
//			"type":  "string",
//			"value": "tracer",
//		},
//		{
//			"key":   "ip",
//			"type":  "string",
//			"value": "10.0.126.253",
//		},
//		{
//			"key":   "opencensus.exporterversion",
//			"type":  "string",
//			"value": "Jaeger-Go-2.30.0",
//		},
//	},
func newLineParse() lineParser {
	return lineParser{}
}

const (
	constructor = iota
	destructor
	info
)

type parsedLine struct {
	timestamp              int64
	typeOfLine             int // represents the line type, eg. constructor, destructor, info line.
	funcName               string
	fileName               string
	spanID                 string // represents the unique id for func
	infoLogs               []string
	directChildParsedLines []*parsedLine
	traceHandle            string
	traceLevel             string
}

type lineParser struct{}

func (l *lineParser) parseLine(line string) (parsed *parsedLine) {
	// eg. destructor: 2023-10-09 13:43:52.695432 40884 StrataTcamProfileSm  8 defaultProfileName_/src/StrataTcamSdkBaseTypes/ProfileSm.tin destructor
	// eg. constructor: 2023-10-09 13:43:52.695442 40884 StrataTcamProfileSm  8 defaultProfileName_/src/StrataTcamSdkBaseTypes/ProfileSm.tin constructor
	// eg. info line: 2023-10-09 13:43:52.695425 40884 StrataTcamProfileSm  8 defaultProfileName
	parts := strings.Fields(line)
	size := len(parts)
	parsed = &parsedLine{}
	switch {
	case isConstructor(line):
		parsed.typeOfLine = constructor
	case isDestructor(line):
		parsed.typeOfLine = destructor
	default:
		parsed.typeOfLine = info
	}

	parsed.timestamp = l.parseTime(parts[0] + parts[1])
	parsed.traceHandle = parts[3]
	parsed.traceLevel = parts[4]

	// Parse func name and file name from func signature
	switch parsed.typeOfLine {
	case constructor, destructor:
		parsed.funcName, parsed.fileName = l.parseFuncSignature(parts[size-2])
	case info:
		parsed.infoLogs = []string{strings.Join(parts[5:], " ")}
	}
	return
}

func (l *lineParser) parseTime(timeStr string) int64 {
	// Define the time string
	// timeStr := "2023-10-07 12:53:59.257452"

	// Parse the time string into a time.Time object
	t, err := time.Parse("2006-01-0215:04:05.999999", timeStr)
	if err != nil {
		fmt.Println("Error parsing time:", err)
		return 0
	}

	// Calculate the microseconds since 1970
	microseconds := t.UnixMicro()

	// fmt.Println("Microseconds since 1970:", microseconds)
	return microseconds
}

func (l *lineParser) parseFuncSignature(funcSignature string) (funcName string, fileName string) {
	separator := "_"
	out := strings.SplitN(funcSignature, separator, 2)
	return out[0], out[1]
}

// Stack represents a stack data structure.
type Stack struct {
	items []*parsedLine
}

// Push adds an item to the top of the stack.
func (s *Stack) Push(item *parsedLine) {
	s.items = append(s.items, item)
}

// Pop removes and returns the item from the top of the stack.
func (s *Stack) Pop() (*parsedLine, error) {
	if len(s.items) == 0 {
		return nil, errors.New("stack is empty")
	}

	index := len(s.items) - 1
	item := s.items[index]
	s.items = s.items[:index]
	return item, nil
}

// Peek returns the item from the top of the stack without removing it.
func (s *Stack) Top() *parsedLine {
	if len(s.items) == 0 {
		log.Fatal(errors.New("unexpected peek"))
		// return nil, errors.New("stack is empty")
	}

	return s.items[len(s.items)-1]
}

// IsEmpty checks if the stack is empty.
func (s *Stack) IsEmpty() bool {
	return len(s.items) == 0
}

// Size returns the number of items in the stack.
func (s *Stack) Size() int {
	return len(s.items)
}
