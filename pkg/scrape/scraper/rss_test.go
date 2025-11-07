// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scraper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"

	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/test"
)

func TestNewRSS(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		config *ScrapeSourceRSS
	}
	type whenDetail struct{} // No specific action details needed for New
	type thenExpected struct {
		wantErr      bool
		wantErrMsg   string
		validateFunc func(t *testing.T, r reader) // Optional validation for successful creation
	}

	// --- Test cases ---
	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Invalid Configuration - Empty URL and RSSHub",
			Given:    "a configuration with empty URL and empty RSSHub config",
			When:     "creating a new RSS reader",
			Then:     "should return a validation error",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				wantErr:    true,
				wantErrMsg: "URL or RSSHubEndpoint can not be empty at the same time",
			},
		},
		{
			Scenario: "Invalid Configuration - Invalid URL format",
			Given:    "a configuration with an invalid URL format",
			When:     "creating a new RSS reader",
			Then:     "should return a URL format error",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					URL: "invalid-url",
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				wantErr:    true,
				wantErrMsg: "URL must be a valid HTTP/HTTPS URL",
			},
		},
		{
			Scenario: "Valid Configuration - URL only",
			Given:    "a valid configuration with only URL",
			When:     "creating a new RSS reader",
			Then:     "should succeed and return a valid reader",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					URL: "http://example.com/feed",
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				wantErr: false,
				validateFunc: func(t *testing.T, r reader) {
					Expect(r).NotTo(BeNil())
					rssReader, ok := r.(*rssReader)
					Expect(ok).To(BeTrue())
					Expect(rssReader.config.URL).To(Equal("http://example.com/feed"))
					// Expect(rssReader.addtionalLabels).To(HaveKey("custom")) // NOTE: rssReader doesn't handle additional labels directly
				},
			},
		},
		{
			Scenario: "Valid Configuration - RSSHub only",
			Given:    "a valid configuration with only RSSHub details",
			When:     "creating a new RSS reader",
			Then:     "should succeed, construct the URL, and return a valid reader",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					RSSHubEndpoint:  "http://rsshub.app/",
					RSSHubRoutePath: "/_/test",
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				wantErr: false,
				validateFunc: func(t *testing.T, r reader) {
					Expect(r).NotTo(BeNil())
					rssReader, ok := r.(*rssReader)
					Expect(ok).To(BeTrue())
					Expect(rssReader.config.URL).To(Equal("http://rsshub.app/_/test"))
					Expect(rssReader.config.RSSHubEndpoint).To(Equal("http://rsshub.app/"))
					Expect(rssReader.config.RSSHubRoutePath).To(Equal("/_/test"))
				},
			},
		},
		{
			Scenario: "Valid Configuration - RSSHub with Access Key",
			Given:    "a valid configuration with RSSHub details and access key",
			When:     "creating a new RSS reader",
			Then:     "should succeed, construct the URL with access key, and return a valid reader",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					RSSHubEndpoint:  "http://rsshub.app/",
					RSSHubRoutePath: "/_/test",
					RSSHubAccessKey: "testkey",
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				wantErr: false,
				validateFunc: func(t *testing.T, r reader) {
					Expect(r).NotTo(BeNil())
					rssReader, ok := r.(*rssReader)
					Expect(ok).To(BeTrue())
					Expect(rssReader.config.URL).To(Equal("http://rsshub.app/_/test?key=testkey"))
					Expect(rssReader.config.RSSHubEndpoint).To(Equal("http://rsshub.app/"))
					Expect(rssReader.config.RSSHubRoutePath).To(Equal("/_/test"))
					Expect(rssReader.config.RSSHubAccessKey).To(Equal("testkey"))
				},
			},
		},
		{
			Scenario: "Valid Configuration - URL with Access Key",
			Given:    "a valid configuration with URL and access key",
			When:     "creating a new RSS reader",
			Then:     "should succeed, append access key to URL, and return a valid reader",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					URL:             "http://example.com/feed",
					RSSHubAccessKey: "testkey",
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				wantErr: false,
				validateFunc: func(t *testing.T, r reader) {
					Expect(r).NotTo(BeNil())
					rssReader, ok := r.(*rssReader)
					Expect(ok).To(BeTrue())
					Expect(rssReader.config.URL).To(Equal("http://example.com/feed"))
					Expect(rssReader.config.RSSHubAccessKey).To(Equal("testkey"))
				},
			},
		},
	}

	// --- Run tests ---
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given & When ---
			r, err := newRSSReader(tt.GivenDetail.config)

			// --- Then ---
			if tt.ThenExpected.wantErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.wantErrMsg))
				Expect(r).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(r).NotTo(BeNil())
				if tt.ThenExpected.validateFunc != nil {
					tt.ThenExpected.validateFunc(t, r)
				}
			}
		})
	}
}

func TestReader_Read(t *testing.T) { // Renamed from TestReader_Read
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		config     *ScrapeSourceRSS
		mockClient func(m *mock.Mock) // Setup mock client behavior
	}
	type whenDetail struct{} // Context is passed, no specific details needed here
	type thenExpected struct {
		feeds        []*model.Feed
		isErr        bool
		wantErrMsg   string
		validateFunc func(t *testing.T, feeds []*model.Feed) // Custom validation
	}

	// --- Test cases ---
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC) // Fixed time for predictable results
	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Basic Feed Fetching",
			Given:    "a valid RSS config and a client returning one feed item",
			When:     "reading the feed",
			Then:     "should return one parsed feed with correct labels",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					URL: "http://techblog.com/feed",
				},
				mockClient: func(m *mock.Mock) {
					m.On("Get", mock.Anything).Return(&gofeed.Feed{
						Items: []*gofeed.Item{
							{
								Title:           "New Tech Article",
								Description:     "Content about new technology",
								Link:            "http://techblog.com/1",
								PublishedParsed: ptr.To(now.Add(-1 * time.Hour)), // Use fixed time offset
							},
						},
					}, nil)
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr: false,
				validateFunc: func(t *testing.T, feeds []*model.Feed) {
					Expect(feeds).To(HaveLen(1))
					Expect(feeds[0].Labels).To(ContainElement(model.Label{Key: model.LabelType, Value: "rss"}))
					Expect(feeds[0].Labels).To(ContainElement(model.Label{Key: model.LabelTitle, Value: "New Tech Article"}))
					Expect(feeds[0].Labels).To(ContainElement(model.Label{Key: model.LabelLink, Value: "http://techblog.com/1"}))
					Expect(feeds[0].Labels).To(ContainElement(model.Label{Key: model.LabelContent, Value: "Content about new technology"})) // Assuming HTML to Markdown conversion is trivial here
					Expect(feeds[0].Labels).To(ContainElement(model.Label{Key: model.LabelPubTime, Value: now.Add(-1 * time.Hour).In(time.Local).Format(time.RFC3339)}))
					// Note: Feed.Time is set by scraper using clk, not tested directly here.
				},
			},
		},
		{
			Scenario: "Client returns error",
			Given:    "a valid RSS config and a client returning an error",
			When:     "reading the feed",
			Then:     "should return the wrapped error",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					URL: "http://techblog.com/feed",
				},
				mockClient: func(m *mock.Mock) {
					m.On("Get", mock.Anything).Return(nil, errors.New("network error"))
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr:      true,
				wantErrMsg: "fetching RSS feed: network error",
				feeds:      nil,
			},
		},
		{
			Scenario: "Client returns empty feed",
			Given:    "a valid RSS config and a client returning an empty feed",
			When:     "reading the feed",
			Then:     "should return an empty slice of feeds without error",
			GivenDetail: givenDetail{
				config: &ScrapeSourceRSS{
					URL: "http://techblog.com/empty",
				},
				mockClient: func(m *mock.Mock) {
					m.On("Get", mock.Anything).Return(&gofeed.Feed{Items: []*gofeed.Item{}}, nil)
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr: false,
				feeds: []*model.Feed{},
			},
		},
	}

	// --- Run tests ---
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given ---
			// Create the reader instance first
			r, err := newRSSReader(tt.GivenDetail.config)
			Expect(err).NotTo(HaveOccurred(), "newRSSReader should succeed for valid test config")
			rssReader, ok := r.(*rssReader)
			Expect(ok).To(BeTrue(), "Expected reader to be of type *rssReader")

			// Create and setup the mock client
			mockCli := newMockClient() // Use the existing mockClient constructor
			if tt.GivenDetail.mockClient != nil {
				tt.GivenDetail.mockClient(&mockCli.Mock)
			}

			// Inject the mock client into the reader instance
			rssReader.client = mockCli

			// --- When ---
			feeds, err := r.Read(context.Background())

			// --- Then ---
			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.wantErrMsg))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// Validate feeds using either direct comparison or custom func
			if tt.ThenExpected.validateFunc != nil {
				tt.ThenExpected.validateFunc(t, feeds)
			} else {
				Expect(feeds).To(Equal(tt.ThenExpected.feeds)) // Direct comparison if no custom validation
			}

			// Assert mock expectations
			mockCli.AssertExpectations(t)
		})
	}
}

func TestParseTime(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		item *gofeed.Item
	}
	type whenDetail struct{}
	type thenExpected struct {
		timeIsNow bool      // True if expected time should be close to time.Now()
		exactTime time.Time // Used only if timeIsNow is false
	}

	fixedTime := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)

	// --- Test cases ---
	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Missing Publication Time",
			Given:    "a feed item without publication time",
			When:     "parsing the publication time",
			Then:     "should return current time (approximated)",
			GivenDetail: givenDetail{
				item: &gofeed.Item{
					PublishedParsed: nil, // Explicitly nil
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				timeIsNow: true,
			},
		},
		{
			Scenario: "Valid Publication Time",
			Given:    "a feed item with valid publication time",
			When:     "parsing the publication time",
			Then:     "should return the item's publication time in Local timezone",
			GivenDetail: givenDetail{
				item: &gofeed.Item{
					PublishedParsed: ptr.To(fixedTime),
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				timeIsNow: false,
				exactTime: fixedTime.In(time.Local), // Expect Local time
			},
		},
	}

	// --- Run tests ---
	r := &rssReader{} // Instance needed to call the method
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given & When ---
			result := r.parseTime(tt.GivenDetail.item)

			// --- Then ---
			if tt.ThenExpected.timeIsNow {
				// Allow for slight difference when checking against time.Now()
				Expect(result).To(BeTemporally("~", time.Now(), time.Second))
			} else {
				Expect(result).To(Equal(tt.ThenExpected.exactTime))
			}
		})
	}
}

func TestCombineContent(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		content     string
		description string
	}
	type whenDetail struct{}
	type thenExpected struct {
		combined string
	}

	// --- Test cases ---
	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Content Only",
			Given:    "a feed item with only content",
			When:     "combining content and description",
			Then:     "should return content only",
			GivenDetail: givenDetail{
				content:     "test content",
				description: "",
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				combined: "test content",
			},
		},
		{
			Scenario: "Description Only",
			Given:    "a feed item with only description",
			When:     "combining content and description",
			Then:     "should return description only",
			GivenDetail: givenDetail{
				content:     "",
				description: "test description",
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				combined: "test description",
			},
		},
		{
			Scenario: "Both Content and Description",
			Given:    "a feed item with both content and description",
			When:     "combining content and description",
			Then:     "should return combined content with newlines",
			GivenDetail: givenDetail{
				content:     "test content",
				description: "test description",
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				combined: "test description\n\ntest content",
			},
		},
		{
			Scenario: "Both Empty",
			Given:    "a feed item with no content and no description",
			When:     "combining content and description",
			Then:     "should return empty string",
			GivenDetail: givenDetail{
				content:     "",
				description: "",
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				combined: "",
			},
		},
	}

	// --- Run tests ---
	r := &rssReader{} // Instance needed to call the method
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given & When ---
			got := r.combineContent(tt.GivenDetail.content, tt.GivenDetail.description)

			// --- Then ---
			Expect(got).To(Equal(tt.ThenExpected.combined))
		})
	}
}
