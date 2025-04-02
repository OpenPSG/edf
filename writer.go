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

	// Write version, patient and recording IDs
	_, err = writer.WriteString(fmt.Sprintf("%-8s", ew.hdr.Version))
	if err != nil {
		return err
	}
	_, err = writer.WriteString(fmt.Sprintf("%-80s", ew.hdr.PatientID))
	if err != nil {
		return err
	}
	_, err = writer.WriteString(fmt.Sprintf("%-80s", ew.hdr.RecordingID))
	if err != nil {
		return err
	}

	// Write start date and time
	dateStr := ew.hdr.StartTime.Format("02.01.06")
	timeStr := ew.hdr.StartTime.Format("15.04.05")
	_, err = writer.WriteString(fmt.Sprintf("%-8s", dateStr))
	if err != nil {
		return err
	}
	_, err = writer.WriteString(fmt.Sprintf("%-8s", timeStr))
	if err != nil {
		return err
	}

	// Write header bytes, data records, etc.
	ew.hdr.HeaderBytes = 256 + (ew.hdr.SignalCount * 256)
	_, err = writer.WriteString(fmt.Sprintf("%-8d", ew.hdr.HeaderBytes))
	if err != nil {
		return err
	}

	// Write 44 empty reserved bytes.
	_, err = writer.WriteString(fmt.Sprintf("%-44s", ""))
	if err != nil {
		return err
	}

	// Write the number of data records.
	_, err = writer.WriteString(fmt.Sprintf("%-8d", ew.hdr.DataRecords))
	if err != nil {
		return err
	}

	// Write data record duration
	_, err = writer.WriteString(fmt.Sprintf("%-8d", int(math.Ceil(ew.hdr.DataRecordDuration.Seconds()))))
	if err != nil {
		return err
	}

	// Write signal count
	_, err = writer.WriteString(fmt.Sprintf("%-4d", ew.hdr.SignalCount))
	if err != nil {
		return err
	}

	// Write signal details
	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-16s", signal.Label))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-80s", signal.TransducerType))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-8s", signal.PhysicalDimension))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(formatPhysicalValue(signal.PhysicalMin))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(formatPhysicalValue(signal.PhysicalMax))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-8d", signal.DigitalMin))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-8d", signal.DigitalMax))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-80s", signal.Prefiltering))
		if err != nil {
			return err
		}
	}

	for _, signal := range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-8d", signal.SamplesPerRecord))
		if err != nil {
			return err
		}
	}

	// Reserved for future use
	for range ew.hdr.Signals {
		_, err = writer.WriteString(fmt.Sprintf("%-32s", ""))
		if err != nil {
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

func formatPhysicalValue(val float64) string {
	// Try with 2 decimal places
	s := fmt.Sprintf("%.2f", val)
	if len(s) > 8 {
		// Fall back to no decimal
		s = fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%-8s", s)
}
