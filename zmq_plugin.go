// Copyright 2017 The fer Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !purego

package fer

import (
	_ "github.com/sbinet-alice/fer/mq/zeromq" // load zeromq plugin
)