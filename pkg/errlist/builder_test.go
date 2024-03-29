// Copyright Istio Authors
//
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

package errlist

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestBuilder(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		g := NewWithT(t)
		errs := Builder{}
		//nolint:ginkgolinter
		g.Expect(errs.Error()).To(BeNil())
	})

	t.Run("single nil error", func(t *testing.T) {
		g := NewWithT(t)
		errs := Builder{}
		errs.Add(nil)
		//nolint:ginkgolinter
		g.Expect(errs.Error()).To(BeNil())
	})

	t.Run("single error", func(t *testing.T) {
		g := NewWithT(t)
		errs := Builder{}
		errs.Add(fmt.Errorf("first error"))
		g.Expect(errs.Error()).To(MatchError("first error"))
	})

	t.Run("multiple errors", func(t *testing.T) {
		g := NewWithT(t)
		errs := Builder{}
		errs.Add(fmt.Errorf("first error"))
		errs.Add(fmt.Errorf("second error"))
		g.Expect(errs.Error()).To(MatchError("first error\nsecond error"))
	})

	t.Run("multiple errors with nils", func(t *testing.T) {
		g := NewWithT(t)
		errs := Builder{}
		errs.Add(nil)
		errs.Add(fmt.Errorf("first error"))
		errs.Add(nil)
		errs.Add(fmt.Errorf("second error"))
		errs.Add(nil)
		g.Expect(errs.Error()).To(MatchError("first error\nsecond error"))
	})

	t.Run("nested", func(t *testing.T) {
		g := NewWithT(t)
		errs := Builder{}
		errs.Add(fmt.Errorf("first error"))

		nestedErrs := Builder{}
		nestedErrs.Add(fmt.Errorf("first nested error"))
		nestedErrs.Add(fmt.Errorf("second nested error"))
		errs.Add(nestedErrs.Error())

		g.Expect(errs.Error()).To(MatchError("first error\nfirst nested error\nsecond nested error"))
	})
}
