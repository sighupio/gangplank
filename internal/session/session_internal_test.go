// Copyright 2017-present SIGHUP s.r.l
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package session

import (
	"testing"
)

func TestGenerateSessionKeys(t *testing.T) {
	t.Parallel()

	b1, b2, err := generateSessionKeys("testing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b1) != 64 || len(b2) != 32 {
		t.Errorf("Wrong byte length's returned")
		return
	}
}
