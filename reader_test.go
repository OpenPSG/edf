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
	"os"
	"testing"

	"github.com/OpenPSG/edf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader(t *testing.T) {
	f, err := os.Open("testdata/resmed_BRP.edf")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	// Parse the header
	er, err := edf.Open(f)
	require.NoError(t, err)

	// Read the first data record
	sr, err := er.Signal(0)
	require.NoError(t, err)

	// Read the first 7500 samples (5 minutes of flow data)
	samples := make([]float64, 7500)
	n, err := sr.Read(samples)
	require.NoError(t, err)
	require.Equal(t, 7500, n)

	// Verify the first 5 samples
	assert.InDelta(t, 0.716, samples[0], 0.001)
	assert.InDelta(t, 0.696, samples[1], 0.001)
	assert.InDelta(t, 0.690, samples[2], 0.001)
	assert.InDelta(t, 0.686, samples[3], 0.001)
	assert.InDelta(t, 0.700, samples[4], 0.001)

	// Verify the last 5 samples
	assert.InDelta(t, -0.272, samples[7495], 0.001)
	assert.InDelta(t, -0.244, samples[7496], 0.001)
	assert.InDelta(t, -0.222, samples[7497], 0.001)
	assert.InDelta(t, -0.212, samples[7498], 0.001)
	assert.InDelta(t, -0.206, samples[7499], 0.001)
}
