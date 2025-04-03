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
	"math"
)

// Writer writes EDF files.
type Writer struct {
	w           io.WriteSeeker
	hdr         *Header
	dataRecords int // Number of data records written so far.
}

// Create creates a new EDF writer that writes to the given writer.
func Create(w io.WriteSeeker, hdr Header) (*Writer, error) {
	hdr.DataRecords = -1 // Unknown number of data records (at this time).

	ew := &Writer{w: w, hdr: &hdr}

	// Write the initial header
	if err := ew.writeHeader(); err != nil {
		return nil, fmt.Errorf("error writing header: %w", err)
	}

	return ew, nil
}

// Close finalizes the EDF file by updating the header with the total number of data records.
func (ew *Writer) Close() error {
	// Finalize the header with the actual number of data records
	ew.hdr.DataRecords = ew.dataRecords
	if err := ew.writeHeader(); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	return nil
}

// WriteRecord writes a single data record to the EDF file.
func (ew *Writer) WriteRecord(signals [][]float64) error {
	if len(signals) != ew.hdr.SignalCount {
		return fmt.Errorf("expected %d signals, got %d", ew.hdr.SignalCount, len(signals))
	}

	var totalSamples int
	for _, signal := range signals {
		totalSamples += len(signal)
	}

	// As recommended by the EDF standard.
	if totalSamples*2 > 61440 {
		return fmt.Errorf("data record too large: %d bytes, max is 61440 bytes", totalSamples*2)
	}

	writer := bufio.NewWriter(ew.w)

	// Write each signal's data
	for i := 0; i < ew.hdr.SignalCount; i++ {
		signal := ew.hdr.Signals[i]
		for _, sample := range signals[i] {
			digitalValue := convertPhysicalToDigital(sample, signal.PhysicalMin, signal.PhysicalMax, signal.DigitalMin, signal.DigitalMax)
			if err := binary.Write(writer, binary.LittleEndian, int16(digitalValue)); err != nil {
				return err
			}
		}
	}

	// Ensure all data is flushed to the underlying writer
	if err := writer.Flush(); err != nil {
		return err
	}

	ew.dataRecords++
	return nil
}

// WriteHeader writes an EDF header to the given writer.
func (ew *Writer) writeHeader() error {
	// Rewind to the beginning of the file.
	_, err := ew.w.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(ew.w)

	writeChecked := func(fieldName, value string, length int) error {
		if len(value) > length {
			return fmt.Errorf("%s '%s' is too long (%d > %d)", fieldName, value, len(value), length)
		}
		_, err := writer.WriteString(fmt.Sprintf("%-*s", length, value))
		return err
	}

	formatPhysicalValue := func(val float64) (string, error) {
		s := fmt.Sprintf("%.2f", val)
		if len(s) > 8 {
			s = fmt.Sprintf("%.0f", val)
			if len(s) > 8 {
				return "", fmt.Errorf("physical value %.2f too long to fit in 8 bytes", val)
			}
		}
		return fmt.Sprintf("%-8s", s), nil
	}

	// Write version, patient and recording IDs
	if err := writeChecked("Version", string(ew.hdr.Version), 8); err != nil {
		return err
	}
	if err := writeChecked("PatientID", ew.hdr.PatientID, 80); err != nil {
		return err
	}
	if err := writeChecked("RecordingID", ew.hdr.RecordingID, 80); err != nil {
		return err
	}

	// Write start date and time
	dateStr := ew.hdr.StartTime.Format("02.01.06")
	timeStr := ew.hdr.StartTime.Format("15.04.05")
	if err := writeChecked("StartDate", dateStr, 8); err != nil {
		return err
	}
	if err := writeChecked("StartTime", timeStr, 8); err != nil {
		return err
	}

	// Write header bytes, data records, etc.
	ew.hdr.HeaderBytes = 256 + (ew.hdr.SignalCount * 256)
	if err := writeChecked("HeaderBytes", fmt.Sprintf("%d", ew.hdr.HeaderBytes), 8); err != nil {
		return err
	}

	// Write 44 empty reserved bytes.
	if err := writeChecked("Reserved", "", 44); err != nil {
		return err
	}

	// Write the number of data records.
	if err := writeChecked("DataRecords", fmt.Sprintf("%d", ew.hdr.DataRecords), 8); err != nil {
		return err
	}

	// Write data record duration
	if err := writeChecked("Duration", fmt.Sprintf("%d", int(math.Ceil(ew.hdr.DataRecordDuration.Seconds()))), 8); err != nil {
		return err
	}

	// Write signal count
	if err := writeChecked("SignalCount", fmt.Sprintf("%d", ew.hdr.SignalCount), 4); err != nil {
		return err
	}

	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].Label", i), signal.Label, 16); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].TransducerType", i), signal.TransducerType, 80); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].PhysicalDimension", i), signal.PhysicalDimension, 8); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		str, err := formatPhysicalValue(signal.PhysicalMin)
		if err != nil {
			return fmt.Errorf("Signal[%d].PhysicalMin: %w", i, err)
		}
		if _, err := writer.WriteString(str); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		str, err := formatPhysicalValue(signal.PhysicalMax)
		if err != nil {
			return fmt.Errorf("Signal[%d].PhysicalMax: %w", i, err)
		}
		if _, err := writer.WriteString(str); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].DigitalMin", i), fmt.Sprintf("%d", signal.DigitalMin), 8); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].DigitalMax", i), fmt.Sprintf("%d", signal.DigitalMax), 8); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].Prefiltering", i), signal.Prefiltering, 80); err != nil {
			return err
		}
	}
	for i, signal := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].SamplesPerRecord", i), fmt.Sprintf("%d", signal.SamplesPerRecord), 8); err != nil {
			return err
		}
	}

	// Reserved for future use
	for i := range ew.hdr.Signals {
		if err := writeChecked(fmt.Sprintf("Signal[%d].Reserved", i), "", 32); err != nil {
			return err
		}
	}

	// Ensure all data is flushed to the underlying writer
	return writer.Flush()
}

// convertPhysicalToDigital converts a physical value to a digital value using the calibration factors.
func convertPhysicalToDigital(physical float64, pmin, pmax float64, dmin, dmax int) int16 {
	if pmax == pmin {
		return 0 // Avoid division by zero
	}
	digital := ((physical - pmin) * (float64(dmax - dmin)) / (pmax - pmin)) + float64(dmin)
	return int16(digital)
}
