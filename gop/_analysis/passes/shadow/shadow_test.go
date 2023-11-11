// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package shadow_test

import (
	"testing"

	"golang.org/x/tools/gop/analysis/analysistest"
	"golang.org/x/tools/gop/analysis/passes/shadow"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, shadow.Analyzer, "a")
}