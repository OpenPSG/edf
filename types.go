// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package edf

import "time"

type Version string

const (
	// Version0 represents the version of the EDF/EDF+ standard.
	Version0 Version = "0"
)

// Header represents the EDF/EDF+ file header.
type Header struct {
	Version            Version       // Version of the EDF/EDF+ standard (usually "0")
	PatientID          string        // Identification of the patient
	RecordingID        string        // Identification of the recording session
	StartTime          time.Time     // Start date of the recording
	HeaderBytes        int           // Number of bytes in the header
	DataRecordDuration time.Duration // Duration of a single data record in seconds
	DataRecords        int           // Number of data records, -1 if unknown
	SignalCount        int           // Number of signals in each data record
	Signals            []Signal      // Details of each signal
}

// Signal represents the characteristics of each signal in the EDF/EDF+ file.
type Signal struct {
	Label             string  // Label of the signal (e.g., EEG Fpz-Cz)
	TransducerType    string  // Type of transducer used
	PhysicalDimension string  // Physical dimension (e.g., uV, mV)
	PhysicalMin       float64 // Minimum physical value
	PhysicalMax       float64 // Maximum physical value
	DigitalMin        int     // Minimum digital value
	DigitalMax        int     // Maximum digital value
	Prefiltering      string  // Pre-filtering information
	SamplesPerRecord  int     // Number of samples in each data record for this signal
	Reserved          string  // Reserved for future use
}
