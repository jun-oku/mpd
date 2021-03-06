// Package mpd implements parsing and generating of MPEG-DASH Media Presentation Description (MPD) files.
package mpd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// http://mpeg.chiariglione.org/standards/mpeg-dash
// https://www.brendanlong.com/the-structure-of-an-mpeg-dash-mpd.html
// http://standards.iso.org/ittf/PubliclyAvailableStandards/MPEG-DASH_schema_files/DASH-MPD.xsd

var emptyElementRE = regexp.MustCompile(`></[A-Za-z]+>`)

// ConditionalUint (ConditionalUintType) defined in XSD as a union of unsignedInt and boolean.
type ConditionalUint struct {
	u *uint64
	b *bool
}

// MarshalXMLAttr encodes ConditionalUint.
func (c ConditionalUint) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	if c.u != nil {
		return xml.Attr{Name: name, Value: strconv.FormatUint(*c.u, 10)}, nil
	}

	if c.b != nil {
		return xml.Attr{Name: name, Value: strconv.FormatBool(*c.b)}, nil
	}

	// both are nil - no attribute, client will threat it like "false"
	return xml.Attr{}, nil
}

// UnmarshalXMLAttr decodes ConditionalUint.
func (c *ConditionalUint) UnmarshalXMLAttr(attr xml.Attr) error {
	u, err := strconv.ParseUint(attr.Value, 10, 64)
	if err == nil {
		c.u = &u
		return nil
	}

	b, err := strconv.ParseBool(attr.Value)
	if err == nil {
		c.b = &b
		return nil
	}

	return fmt.Errorf("ConditionalUint: can't UnmarshalXMLAttr %#v", attr)
}

// check interfaces
var (
	_ xml.MarshalerAttr   = ConditionalUint{}
	_ xml.UnmarshalerAttr = &ConditionalUint{}
)

// MPD represents root XML element.
type MPD struct {
	XMLNS                      *string   `xml:"xmlns,attr"`
	Cenc                       *string   `xml:"cenc,attr"`
	Mspr                       *string   `xml:"mspr,attr"`
	Type                       *string   `xml:"type,attr"`
	MinimumUpdatePeriod        *string   `xml:"minimumUpdatePeriod,attr"`
	AvailabilityStartTime      *string   `xml:"availabilityStartTime,attr"`
	MediaPresentationDuration  *string   `xml:"mediaPresentationDuration,attr"`
	MinBufferTime              *string   `xml:"minBufferTime,attr"`
	SuggestedPresentationDelay *string   `xml:"suggestedPresentationDelay,attr"`
	TimeShiftBufferDepth       *string   `xml:"timeShiftBufferDepth,attr"`
	PublishTime                *string   `xml:"publishTime,attr"`
	Profiles                   string    `xml:"profiles,attr"`
	BaseURL                    string    `xml:"BaseURL,omitempty"`
	Periods                    []*Period `xml:"Period,omitempty"`
}

// Do not try to use encoding.TextMarshaler and encoding.TextUnmarshaler:
// https://github.com/golang/go/issues/6859#issuecomment-118890463

// Encode generates MPD XML.
func (m *MPD) Encode() ([]byte, error) {
	x := new(bytes.Buffer)
	e := xml.NewEncoder(x)
	e.Indent("", "  ")
	err := e.Encode(m)
	if err != nil {
		return nil, err
	}

	// hacks for self-closing tags
	res := new(bytes.Buffer)
	res.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	res.WriteByte('\n')
	for {
		s, err := x.ReadString('\n')
		if s != "" {
			s = emptyElementRE.ReplaceAllString(s, `/>`)
			// namespaceへの対応が必要なためここで書き換える
			// 参考 : https://github.com/golang/go/issues/11496
			if strings.Contains(s, "<MPD") {
				s = strings.Replace(s, "cenc", "xmlns:cenc", 1)
				s = strings.Replace(s, "mspr", "xmlns:mspr", 1)
			}
			if strings.Contains(s, "<pssh") {
				s = strings.Replace(s, "cenc", "xmlns:cenc", 1)
				s = strings.Replace(s, "pssh", "cenc:pssh", -1)
			}
			if strings.Contains(s, "<pro") {
				s = strings.Replace(s, "mspr", "xmlns:mspr", 1)
				s = strings.Replace(s, "pro", "mspr:pro", -1)
			}
			if strings.Contains(s, "default_KID") {
				s = strings.Replace(s, "default_KID", "cenc:default_KID", -1)
				s = strings.Replace(s, "cenc=", "xmlns:cenc=", -1)
			}
			if strings.TrimSpace(s) == "<SegmentTimeline/>" {
				s = ""
			}
			res.WriteString(s)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	res.WriteByte('\n')
	return res.Bytes(), err
}

// Decode parses MPD XML.
func (m *MPD) Decode(b []byte) error {
	return xml.Unmarshal(b, m)
}

// Period represents XSD's PeriodType.
type Period struct {
	Start                *string              `xml:"start,attr"`
	ID                   *string              `xml:"id,attr"`
	Duration             *string              `xml:"duration,attr"`
	SupplementalProperty *Descriptor          `xml:"SupplementalProperty,omitempty"`
	BaseURL              string               `xml:"BaseURL,omitempty"`
	EventStreams         []EventStream        `xml:"EventStream,omitempty"`
	ProgramEventStreams  []ProgramEventStream `xml:"ProgramEventStream,omitempty"`
	AdaptationSets       []*AdaptationSet     `xml:"AdaptationSet,omitempty"`
}

type Descriptor struct {
	SchemeIDURI *string `xml:"schemeIdUri,attr"`
	Value       *string `xml:"value,attr"`
	ID          *string `xml:"id,attr"`
}

// AdaptationSet represents XSD's AdaptationSetType.
type AdaptationSet struct {
	MimeType                string              `xml:"mimeType,attr"`
	SegmentAlignment        ConditionalUint     `xml:"segmentAlignment,attr"`
	SubsegmentAlignment     ConditionalUint     `xml:"subsegmentAlignment,attr"`
	StartWithSAP            *uint64             `xml:"startWithSAP,attr"`
	SubsegmentStartsWithSAP *uint64             `xml:"subsegmentStartsWithSAP,attr"`
	BitstreamSwitching      *bool               `xml:"bitstreamSwitching,attr"`
	Lang                    *string             `xml:"lang,attr"`
	ContentProtections      []ContentProtection `xml:"ContentProtection,omitempty"`
	Representations         []Representation    `xml:"Representation,omitempty"`
	FrameRate               *string             `xml:"frameRate,attr"`
	SegmentTemplate         *SegmentTemplate    `xml:"SegmentTemplate,omitempty"`
}

// Representation represents XSD's RepresentationType.
type Representation struct {
	ID                        *string                    `xml:"id,attr"`
	Width                     *uint64                    `xml:"width,attr"`
	Height                    *uint64                    `xml:"height,attr"`
	FrameRate                 *string                    `xml:"frameRate,attr"`
	Bandwidth                 *uint64                    `xml:"bandwidth,attr"`
	AudioSamplingRate         *string                    `xml:"audioSamplingRate,attr"`
	Codecs                    *string                    `xml:"codecs,attr"`
	ContentProtections        []ContentProtection        `xml:"ContentProtection,omitempty"`
	SegmentTemplate           *SegmentTemplate           `xml:"SegmentTemplate,omitempty"`
	ScanType                  *string                    `xml:"scanType,attr"`
	AudioChannelConfiguration *AudioChannelConfiguration `xml:"AudioChannelConfiguration,omitempty"`
}

// AudioChannelConfiguration,EventStream,Event from github.com/zencoder/go-dash //
type AudioChannelConfiguration struct {
	SchemeIDURI *string `xml:"schemeIdUri,attr"`
	// Value will be an int for non-Dolby Schemes, and a hexstring for Dolby Schemes, hence we make it a string
	Value *string `xml:"value,attr"`
}

// ProgramEventStream represents custom EventStream.
type ProgramEventStream struct {
	XMLName     xml.Name `xml:"ProgramEventStream"`
	SchemeIDURI *string  `xml:"schemeIdUri,attr"`
	Value       *string  `xml:"value,attr,omitempty"`
	Timescale   *int64   `xml:"timescale,attr"`
	Events      []Event  `xml:"Event,omitempty"`
}

// EventStream from github.com/zencoder/go-dash //
type EventStream struct {
	XMLName     xml.Name `xml:"EventStream"`
	SchemeIDURI *string  `xml:"schemeIdUri,attr"`
	Value       *string  `xml:"value,attr,omitempty"`
	Timescale   *int64   `xml:"timescale,attr"`
	Events      []Event  `xml:"Event,omitempty"`
}

// Event from github.com/zencoder/go-dash //
type Event struct {
	XMLName          xml.Name `xml:"Event"`
	ID               *string  `xml:"id,attr,omitempty"`
	PresentationTime *int64   `xml:"presentationTime,attr,omitempty"`
	Duration         *int64   `xml:"duration,attr,omitempty"`
}

// ContentProtection represents XSD's ContentProtectionType.
type ContentProtection struct {
	SchemeIDURI *string `xml:"schemeIdUri,attr"`
	Value       *string `xml:"value,attr"`
	DefaultKID  *string `xml:"default_KID,attr"`
	Cenc        *string `xml:"cenc,attr"`
	Pssh        *Pssh   `xml:"pssh,omitempty"`
	Pro         *Pro    `xml:"pro,omitempty"`
}

// Pssh represents XSD's PsshType.
type Pssh struct {
	Value *string `xml:",chardata"`
	Cenc  *string `xml:"cenc,attr"`
}

// Pro represents XSD's PsshType.
type Pro struct {
	Value *string `xml:",chardata"`
	Mspr  *string `xml:"mspr,attr"`
}

// SegmentTemplate represents XSD's SegmentTemplateType.
type SegmentTemplate struct {
	Timescale              *uint64           `xml:"timescale,attr"`
	Media                  *string           `xml:"media,attr"`
	Initialization         *string           `xml:"initialization,attr"`
	StartNumber            *uint64           `xml:"startNumber,attr"`
	PresentationTimeOffset *uint64           `xml:"presentationTimeOffset,attr"`
	Duration               *uint32           `xml:"duration,attr,omitempty"`
	SegmentTimeline        []SegmentTimeline `xml:"SegmentTimeline,omitempty"`
}

// SegmentTimeline represents XSD's SegmentTimelineType.
type SegmentTimeline struct {
	Segments []SegmentTimelineSegment `xml:"S,omitempty"`
}

// SegmentTimelineSegment represents XSD's SegmentTimelineType's inner S elements.
type SegmentTimelineSegment struct {
	T *uint64 `xml:"t,attr"`
	D uint64  `xml:"d,attr"`
	R *int64  `xml:"r,attr"`
}
