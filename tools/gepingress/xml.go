package main

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"io"
)

type CDeviceDetail struct {
	XMLName              xml.Name `xml:"DeviceDetail,omitempty"`
	COriginalSource      string   `xml:"OriginalSource,omitempty"`
	CAccessID            string   `xml:"AccessID,omitempty"`
	CAcronym             string   `xml:"Acronym,omitempty"`
	CCompanyAcronym      string   `xml:"CompanyAcronym,omitempty"`
	CContactList         string   `xml:"ContactList,omitempty"`
	CEnabled             bool     `xml:"Enabled,omitempty"`
	CFramesPerSecond     int      `xml:"FramesPerSecond,omitempty"`
	CInterconnectionName string   `xml:"InterconnectionName,omitempty"`
	CIsConcentrator      bool     `xml:"IsConcentrator,omitempty"`
	CLatitude            float64  `xml:"Latitude,omitempty"`
	CLongitude           float64  `xml:"Longitude,omitempty"`
	CName                string   `xml:"Name,omitempty"`
	CNodeID              string   `xml:"NodeID,omitempty"`
	CParentAcronym       string   `xml:"ParentAcronym,omitempty"`
	CProtocolName        string   `xml:"ProtocolName,omitempty"`
	CUniqueID            string   `xml:"UniqueID,omitempty"`
	CUpdatedOn           string   `xml:"UpdatedOn,omitempty"`
	CVendorAcronym       string   `xml:"VendorAcronym,omitempty"`
	CVendorDeviceName    string   `xml:"VendorDeviceName,omitempty"`
}

type CMeasurementDetail struct {
	XMLName            xml.Name       `xml:"MeasurementDetail,omitempty"`
	CDescription       string         `xml:"Description,omitempty"`
	CDeviceAcronym     string         `xml:"DeviceAcronym,omitempty"`
	CEnabled           bool           `xml:"Enabled,omitempty"`
	CID                string         `xml:"ID,omitempty"`
	CInternal          bool           `xml:"Internal,omitempty"`
	CPhasorSourceIndex int            `xml:"PhasorSourceIndex,omitempty"`
	CPointTag          string         `xml:"PointTag,omitempty"`
	CSignalAcronym     string         `xml:"SignalAcronym,omitempty"`
	CSignalID          string         `xml:"SignalID,omitempty"`
	CSignalReference   string         `xml:"SignalReference,omitempty"`
	CUpdatedOn         string         `xml:"UpdatedOn,omitempty"`
	PhasorDetail       *CPhasorDetail `xml:"-"`
	DeviceDetail       *CDeviceDetail `xml:"-"`
}

type CNewDataSet struct {
	XMLName            xml.Name              `xml:"NewDataSet,omitempty"`
	CDeviceDetail      []*CDeviceDetail      `xml:"DeviceDetail,omitempty"`
	CMeasurementDetail []*CMeasurementDetail `xml:"MeasurementDetail,omitempty"`
	CPhasorDetail      []*CPhasorDetail      `xml:"PhasorDetail,omitempty"`
}
type CPhasorDetail struct {
	XMLName              xml.Name `xml:"PhasorDetail,omitempty"`
	CDeviceAcronym       string   `xml:"DeviceAcronym,omitempty"`
	CID                  string   `xml:"ID,omitempty"`
	CLabel               string   `xml:"Label,omitempty"`
	CPhase               string   `xml:"Phase,omitempty"`
	CSourceIndex         int      `xml:"SourceIndex,omitempty"`
	CDestinationPhasorID int      `xml:"DestinationPhasorID,omitempty"`
	CType                string   `xml:"Type,omitempty"`
	CUpdatedOn           string   `xml:"UpdatedOn,omitempty"`
}

func ParseXMLMetadata(data []byte) (*CNewDataSet, map[string]*CMeasurementDetail, error) {

	buf := bytes.NewBuffer(data)
	rdr, err := gzip.NewReader(buf)
	if err == nil {
		decompressedBuf := &bytes.Buffer{}
		decompressionOkay := false
		for {
			chunk := make([]byte, 8192)
			n, err := rdr.Read(chunk)
			if err != nil {
				if err == io.EOF {
					decompressedBuf.Write(chunk[:n])
					decompressionOkay = true
					break
				}
				//Probably uncompressed metadata
				break
			}
			decompressedBuf.Write(chunk[:n])
		}
		rdr.Close()
		if decompressionOkay {
			data = decompressedBuf.Bytes()
		}
	}

	rv := &CNewDataSet{}
	err = xml.Unmarshal(data, rv)
	if err != nil {
		return nil, nil, err
	}

	//Populate device detail into the measurement details
	devices := make(map[string]*CDeviceDetail)
	for _, d := range rv.CDeviceDetail {
		devices[d.CAcronym] = d
	}
	for _, m := range rv.CMeasurementDetail {
		d, ok := devices[m.CDeviceAcronym]
		if ok {
			m.DeviceDetail = d
		} else {
			continue
		}
	}

	//Populate the phasor detail into the measurements
	//The SignalReference stuff is weird, consult
	// https://github.com/GridProtectionAlliance/gsf/blob/master/Source/Libraries/TimeSeriesPlatformLibrary/Transport/TransportTypes.cpp#L73-L106
	type pk struct {
		DeviceAcronym string
		SourceIndex   int
	}
	phasors := make(map[pk]*CPhasorDetail)
	for _, p := range rv.CPhasorDetail {
		phasors[pk{p.CDeviceAcronym, p.CSourceIndex}] = p
	}

	//Go through measurements and link phasor details
	for _, m := range rv.CMeasurementDetail {
		if m.CPhasorSourceIndex != 0 {
			phasor := phasors[pk{m.CDeviceAcronym, m.CPhasorSourceIndex}]
			m.PhasorDetail = phasor
		}
	}

	//Create final uuid->meta map
	fmap := make(map[string]*CMeasurementDetail)
	for _, m := range rv.CMeasurementDetail {
		fmap[m.CSignalID] = m
	}

	return rv, fmap, nil
}
