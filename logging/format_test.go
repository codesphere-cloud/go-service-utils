package logging_test

import (
	"github.com/codesphere-cloud/go-service-utils/logging"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReadFormat", func() {
	It("LOG_FORMAT unset -> json", func() {
		GinkgoT().Setenv("LOG_FORMAT", "")

		Expect(logging.ReadFormat()).To(Equal(logging.FormatJSON))
	})

	It("LOG_FORMAT=json -> json", func() {
		GinkgoT().Setenv("LOG_FORMAT", "json")

		Expect(logging.ReadFormat()).To(Equal(logging.FormatJSON))
	})

	It("LOG_FORMAT=text -> text", func() {
		GinkgoT().Setenv("LOG_FORMAT", "text")

		Expect(logging.ReadFormat()).To(Equal(logging.FormatText))
	})

	It("LOG_FORMAT unknown -> error", func() {
		GinkgoT().Setenv("LOG_FORMAT", "jsno")

		_, err := logging.ReadFormat()

		Expect(err).To(MatchError(ContainSubstring("invalid LOG_FORMAT")))
	})
})
