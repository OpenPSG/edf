// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package edf_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenPSG/edf"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	f, err := os.OpenFile(filepath.Join(t.TempDir(), "test.edf"), os.O_RDWR|os.O_CREATE, 0o644)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	hdr := edf.Header{
		Version:            edf.Version0,
		PatientID:          "Patient X",
		RecordingID:        "Recording 1",
		StartTime:          time.Now(),
		DataRecordDuration: 60 * time.Second,
		SignalCount:        1,
		Signals: []edf.Signal{
			{
				Label:             "EEG Fpz-Cz",
				TransducerType:    "AgAgCl electrode",
				PhysicalDimension: "uV",
				PhysicalMin:       -500,
				PhysicalMax:       500,
				DigitalMin:        -2048,
				DigitalMax:        2047,
				SamplesPerRecord:  256,
			},
		},
	}

	ew, err := edf.Create(f, hdr)
	require.NoError(t, err)

	// Write some data records
	record := make([]float64, 256)
	for i := range record {
		record[i] = float64(i) // physical value
	}

	// Write the first data record
	err = ew.WriteRecord([][]float64{record})
	require.NoError(t, err)

	for i := range record {
		record[i] = float64(i + 256)
	}

	// Write the second data record
	err = ew.WriteRecord([][]float64{record})
	require.NoError(t, err)

	// Close the writer (this writes the header)
	require.NoError(t, ew.Close())

	// Rewind the file
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	// Read the file
	er, err := edf.Open(f)
	require.NoError(t, err)

	// Read the first data record
	sr, err := er.Signal(0)
	require.NoError(t, err)

	// Read the first 511 samples
	samples := make([]float64, 512)
	n, err := sr.Read(samples)
	require.NoError(t, err)
	require.Equal(t, 512, n)

	// Verify the samples match what was written.
	for i := range samples {
		require.InDelta(t, float64(i), samples[i], 1.0)
	}

	// Reader should now return EOF
	_, err = sr.Read(samples)
	require.Equal(t, io.EOF, err)
}
