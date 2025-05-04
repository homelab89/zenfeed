package rewrite

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/llm"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/test"
)

func TestLabels(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		config  *Config
		llmMock func(m *mock.Mock)
	}
	type whenDetail struct {
		inputLabels model.Labels
	}
	type thenExpected struct {
		outputLabels model.Labels
		err          error
		isErr        bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Drop feed based on transformed content match",
			Given:    "a rule to drop feed if transformed content matches 'spam'",
			When:     "processing labels where transformed content is 'spam'",
			Then:     "should return nil labels indicating drop",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:   ToTextTypePrompt,
								LLM:    "mock-llm",
								Prompt: "{{ .category }}", // Using a simple template for testing
							},
						},
						Match:  "spam",
						Action: ActionDropFeed,
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("spam", nil)
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "This is some content that will be transformed to spam."},
					{Key: model.LabelTitle, Value: "Spam Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: nil,
				isErr:        false,
			},
		},
		{
			Scenario: "Create/Update label based on transformed content",
			Given:    "a rule to add a category label based on transformed content",
			When:     "processing labels where transformed content is 'Technology'",
			Then:     "should return labels with the new category label",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:   ToTextTypePrompt,
								LLM:    "mock-llm",
								Prompt: "{{ .category }}",
							},
						},
						Match:  "Technology",
						Action: ActionCreateOrUpdateLabel,
						Label:  "category",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("Technology", nil)
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about AI and programming."},
					{Key: model.LabelTitle, Value: "Tech Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about AI and programming."},
					{Key: model.LabelTitle, Value: "Tech Article"},
					{Key: "category", Value: "Technology"},
				},
				isErr: false,
			},
		},
		{
			Scenario: "No rules match",
			Given:    "a rule that does not match the content",
			When:     "processing labels",
			Then:     "should return the original labels unchanged",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Match:                 "NonMatchingPattern",
						Action:                ActionDropFeed,
					},
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Some regular content."},
					{Key: model.LabelTitle, Value: "Regular Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Some regular content."},
					{Key: model.LabelTitle, Value: "Regular Article"},
				},
				isErr: false,
			},
		},
		{
			Scenario: "LLM transformation error",
			Given:    "a rule requiring transformation and LLM returns an error",
			When:     "processing labels",
			Then:     "should return an error",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:           ToTextTypePrompt,
								LLM:            "mock-llm",
								Prompt:         "{{ .category }}",
								promptRendered: "Analyze the content and categorize it...",
							},
						},
						Match:  ".*",
						Action: ActionCreateOrUpdateLabel,
						Label:  "category",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("", errors.New("LLM failed"))
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content requiring transformation."},
					{Key: model.LabelTitle, Value: "Transform Error Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: nil,
				err:          errors.New("transform text: llm completion: LLM failed"),
				isErr:        true,
			},
		},
		{
			Scenario: "Rule matches but label already exists",
			Given:    "a rule to add a category label and the label already exists",
			When:     "processing labels",
			Then:     "should update the existing label value",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:           ToTextTypePrompt,
								LLM:            "mock-llm",
								Prompt:         "{{ .category }}",
								promptRendered: "Analyze the content and categorize it...",
							},
						},
						Match:  "Finance",
						Action: ActionCreateOrUpdateLabel,
						Label:  "category",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("Finance", nil)
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about stock market."},
					{Key: model.LabelTitle, Value: "Finance Article"},
					{Key: "category", Value: "OldCategory"}, // Existing label
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about stock market."},
					{Key: model.LabelTitle, Value: "Finance Article"},
					{Key: "category", Value: "Finance"}, // Updated label
				},
				isErr: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			var mockLLMFactory llm.Factory
			var mockInstance *mock.Mock // Store the mock instance for assertion

			// Create mock factory and capture the mock.Mock instance.
			mockOption := component.MockOption(func(m *mock.Mock) {
				mockInstance = m // Capture the mock instance.
				if tt.GivenDetail.llmMock != nil {
					tt.GivenDetail.llmMock(m)
				}
			})
			mockLLMFactory, err := llm.NewFactory("", nil, llm.FactoryDependencies{}, mockOption) // Use the factory directly with the option
			Expect(err).NotTo(HaveOccurred())

			// Manually validate config to compile regex and render templates.
			// In real usage, this happens in `new` or `Reload`.
			for i := range *tt.GivenDetail.config {
				err := (*tt.GivenDetail.config)[i].Validate()
				Expect(err).NotTo(HaveOccurred(), "Rule validation should not fail in test setup")
			}

			// Instantiate the rewriter with the mock factory
			rewriterInstance := &rewriter{
				Base: component.New(&component.BaseConfig[Config, Dependencies]{
					Name:     "TestRewriter",
					Instance: "test",
					Config:   tt.GivenDetail.config,
					Dependencies: Dependencies{
						LLMFactory: mockLLMFactory, // Pass the mock factory
					},
				}),
			}

			// Clone input labels to avoid modification by reference affecting assertions.
			inputLabelsCopy := make(model.Labels, len(tt.WhenDetail.inputLabels))
			copy(inputLabelsCopy, tt.WhenDetail.inputLabels)

			// When.
			outputLabels, err := rewriterInstance.Labels(context.Background(), inputLabelsCopy)

			// Then.
			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				// Use MatchError for potentially wrapped errors.
				Expect(err).To(MatchError(ContainSubstring(tt.ThenExpected.err.Error())))
				Expect(outputLabels).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				// Ensure output labels are sorted for consistent comparison.
				if outputLabels != nil {
					outputLabels.EnsureSorted()
				}
				tt.ThenExpected.outputLabels.EnsureSorted()
				Expect(outputLabels).To(Equal(tt.ThenExpected.outputLabels))
			}

			// Verify LLM calls if stubs were provided.
			if tt.GivenDetail.llmMock != nil && mockInstance != nil {
				// Assert expectations on the captured mock instance.
				mockInstance.AssertExpectations(t)
			}
		})
	}
}
