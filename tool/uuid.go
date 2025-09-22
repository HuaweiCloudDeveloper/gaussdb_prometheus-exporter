// Copyright 2021 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tool

import (
	"encoding/hex"
	"fmt"
)

// decodeUUIDBinary interprets the binary format of a uuid, returning it in text format.
func decodeUUIDBinary(src []byte) ([]byte, error) {
	if len(src) != 16 {
		return nil, fmt.Errorf("pq: unable to decode uuid; bad length: %d", len(src))
	}

	dst := make([]byte, 36)
	dst[8], dst[13], dst[18], dst[23] = '-', '-', '-', '-'
	hex.Encode(dst[0:], src[0:4])
	hex.Encode(dst[9:], src[4:6])
	hex.Encode(dst[14:], src[6:8])
	hex.Encode(dst[19:], src[8:10])
	hex.Encode(dst[24:], src[10:16])

	return dst, nil
}
