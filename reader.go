// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package edf

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Reader reads EDF/EDF+ files.
type Reader struct {
	r   io.ReadSeeker
	hdr *Header
}

// Open opens an EDF/EDF+ file for reading.
func Open(r io.ReadSeeker) (*Reader, error) {
	reader := bufio.NewReader(r)

	b := make([]byte, 256)
	if _, err := io.ReadFull(reader, b); err != nil {
		return nil, fmt.Errorf("error reading header: %w", err)
	}

	// Parse fields based on EDF/EDF+ specifications
	hdr := &Header{}
	hdr.Version = Version(strings.TrimSpace(string(b[0:8])))
	hdr.PatientID = strings.TrimSpace(string(b[8:88]))
	hdr.RecordingID = strings.TrimSpace(string(b[88:168]))
	dateStr := strings.TrimSpace(string(b[168:176]))
	timeStr := strings.TrimSpace(string(b[176:184]))

	// Parse start date and time
	var err error
	startDate, err := time.Parse("02.01.06", dateStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing start date: %w", err)
	}
	startTime, err := time.Parse("15.04.05", timeStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing start time: %w", err)
	}
	hdr.StartTime = time.Date(startDate.Year(), startDate.Month(), startDate.Day(),
		startTime.Hour(), startTime.Minute(), startTime.Second(), 0, time.UTC)

	// Continue reading header to get number of data records, duration of data records, etc.
	headerBytes, err := strconv.Atoi(strings.TrimSpace(string(b[184:192])))
	if err != nil {
		return nil, fmt.Errorf("error parsing header bytes: %w", err)
	}
	hdr.HeaderBytes = headerBytes

	numDataRecords, err := strconv.Atoi(strings.TrimSpace(string(b[236:244])))
	if err != nil {
		return nil, fmt.Errorf("error parsing number of data records: %w", err)
	}
	hdr.DataRecords = numDataRecords

	hdr.DataRecordDuration, err = time.ParseDuration(fmt.Sprintf("%ss", strings.TrimSpace(string(b[244:252]))))
	if err != nil {
		return nil, fmt.Errorf("error parsing data record duration: %w", err)
	}

	signalCount, err := strconv.Atoi(strings.TrimSpace(string(b[252:256])))
	if err != nil {
		return nil, fmt.Errorf("error parsing signal count: %w", err)
	}
	hdr.SignalCount = signalCount

	// Read signal headers
	hdr.Signals = make([]Signal, signalCount)

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 16)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].Label = strings.TrimSpace(string(b))
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 80)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].TransducerType = strings.TrimSpace(string(b))
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 8)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].PhysicalDimension = strings.TrimSpace(string(b))
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 8)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].PhysicalMin = parseFloat(b)
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 8)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].PhysicalMax = parseFloat(b)
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 8)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].DigitalMin = parseInt(b)
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 8)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].DigitalMax = parseInt(b)
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 80)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].Prefiltering = strings.TrimSpace(string(b))
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 8)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].SamplesPerRecord = parseInt(b)
	}

	for i := 0; i < signalCount; i++ {
		b := make([]byte, 32)
		if _, err := io.ReadFull(reader, b); err != nil {
			return nil, fmt.Errorf("error reading signal headers: %w", err)
		}

		hdr.Signals[i].Reserved = strings.TrimSpace(string(b))
	}

	return &Reader{
		r:   r,
		hdr: hdr,
	}, nil
}

// SignalReader reads continuous signal data from an EDF/EDF+ file.
type SignalReader struct {
	r                io.ReadSeeker
	hdr              *Header
	signalIndex      int // Index of the signal to read
	currentRecord    int // Current record being processed
	currentSample    int // Current sample in the record
	recordSize       int // Total size of one data record
	signalOffset     int // Byte offset of the signal in a record
	samplesPerRecord int // Number of samples per record for the signal
}

// Signal creates a new SignalReader for a specified signal index.
func (er *Reader) Signal(signalIndex int) (*SignalReader, error) {
	if signalIndex < 0 || signalIndex >= len(er.hdr.Signals) {
		return nil, fmt.Errorf("signal index out of range")
	}

	signal := er.hdr.Signals[signalIndex]
	recordSize := 0
	signalOffset := 0
	for i, sig := range er.hdr.Signals {
		if i < signalIndex {
			signalOffset += sig.SamplesPerRecord * 2
		}
		recordSize += sig.SamplesPerRecord * 2
	}

	return &SignalReader{
		r:                er.r,
		hdr:              er.hdr,
		signalIndex:      signalIndex,
		recordSize:       recordSize,
		signalOffset:     signalOffset,
		samplesPerRecord: signal.SamplesPerRecord,
	}, nil
}

// Read fills the provided float64 slice with the physical values from the signal.
func (sr *SignalReader) Read(data []float64) (int, error) {
	buf := make([]byte, 2)

	n := 0
	for n < len(data) {
		if sr.currentRecord >= sr.hdr.DataRecords {
			return n, io.EOF // End of data records
		}

		// Calculate position to read the digital sample from
		pos := int64(sr.hdr.HeaderBytes) + int64(sr.currentRecord)*int64(sr.recordSize) + int64(sr.signalOffset) + int64(sr.currentSample*2)
		if _, err := sr.r.Seek(pos, io.SeekStart); err != nil {
			return n, fmt.Errorf("error seeking to position: %w", err)
		}

		// Read the digital sample
		if _, err := io.ReadFull(sr.r, buf); err != nil {
			return n, fmt.Errorf("error reading sample data: %w", err)
		}
		digitalValue := int16(binary.LittleEndian.Uint16(buf))
		signal := sr.hdr.Signals[sr.signalIndex]
		data[n] = convertDigitalToPhysical(digitalValue, signal.DigitalMin, signal.DigitalMax, signal.PhysicalMin, signal.PhysicalMax)

		n++

		// Move to the next sample
		sr.currentSample++
		if sr.currentSample >= sr.samplesPerRecord {
			sr.currentSample = 0
			sr.currentRecord++
		}
	}

	return n, nil
}

// convertDigitalToPhysical converts a digital value from the data record to a physical value using the calibration factors.
func convertDigitalToPhysical(digital int16, dmin, dmax int, pmin, pmax float64) float64 {
	if dmax == dmin {
		return 0 // Avoid division by zero
	}
	return pmin + (float64(digital)-float64(dmin))*(pmax-pmin)/float64(dmax-dmin)
}

func parseFloat(b []byte) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return 0.0
	}
	return f
}

func parseInt(b []byte) int {
	i, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return i
}
