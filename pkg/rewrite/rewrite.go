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

package rewrite

import (
	"context"
	"html/template"
	"regexp"
	"unicode/utf8"
	"unsafe"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/llm"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	"github.com/glidea/zenfeed/pkg/util/buffer"
	crawlutil "github.com/glidea/zenfeed/pkg/util/crawl"
)

// --- Interface code block ---

type Rewriter interface {
	component.Component
	config.Watcher

	// Labels applies rewrite rules to the given labels and returns the modified labels.
	// Note: this method modifies the input labels in place.
	// If a rule's action is ActionDropFeed, it returns nil to indicate the item should be dropped.
	Labels(ctx context.Context, labels model.Labels) (model.Labels, error)
}

type Config []Rule

func (c *Config) Validate() error {
	for i := range *c {
		if err := (*c)[i].Validate(); err != nil {
			return errors.Wrapf(err, "validate and adjust rewrite config")
		}
	}

	return nil
}

func (c *Config) From(app *config.App) {
	for _, r := range app.Storage.Feed.Rewrites {
		var rc Rule
		rc.From(&r)
		*c = append(*c, rc)
	}
}

type Dependencies struct {
	LLMFactory llm.Factory
}

type Rule struct {
	// SourceLabel specifies which label's value to use as source text.
	// Default is model.LabelContent.
	SourceLabel string

	// SkipTooShortThreshold is the threshold of the source text length.
	// If the source text is shorter than this threshold, it will be skipped.
	SkipTooShortThreshold *int

	// Transform used to transform the source text.
	// If not set, transform to original source text.
	Transform *Transform

	// Match used to match the text after transform.
	// If not set, match all.
	Match   string
	matchRE *regexp.Regexp

	// Action determines what to do if matchs.
	Action Action

	// Label is the label to create or update.
	Label string
}

func (r *Rule) Validate() error { //nolint:cyclop
	// Source label.
	if r.SourceLabel == "" {
		r.SourceLabel = model.LabelContent
	}
	if r.SkipTooShortThreshold == nil {
		r.SkipTooShortThreshold = ptr.To(300)
	}

	// Transform.
	if r.Transform != nil {
		switch r.Transform.ToText.Type {
		case ToTextTypePrompt, ToTextTypeScrape:
		default:
			return errors.Errorf("unknown transform type: %s", r.Transform.ToText.Type)
		}

		switch r.Transform.ToText.Type {
		case ToTextTypePrompt:
			if r.Transform.ToText.Prompt == "" {
				return errors.New("to text prompt is required")
			}
			tmpl, err := template.New("").Parse(r.Transform.ToText.Prompt)
			if err != nil {
				return errors.Wrapf(err, "parse prompt template %s", r.Transform.ToText.Prompt)
			}
			buf := buffer.Get()
			defer buffer.Put(buf)
			if err := tmpl.Execute(buf, promptTemplates); err != nil {
				return errors.Wrapf(err, "execute prompt template %s", r.Transform.ToText.Prompt)
			}
			r.Transform.ToText.promptRendered = buf.String()
		}
	}

	// Match.
	if r.Match == "" {
		r.Match = ".*"
	}
	re, err := regexp.Compile(r.Match)
	if err != nil {
		return errors.Wrapf(err, "compile match regex %s", r.Match)
	}
	r.matchRE = re

	// Action.
	switch r.Action {
	case "":
		r.Action = ActionCreateOrUpdateLabel
	case ActionCreateOrUpdateLabel:
		if r.Label == "" {
			return errors.New("label is required for create or update label action")
		}
	case ActionDropFeed:
	default:
		return errors.Errorf("invalid action: %s", r.Action)
	}

	return nil
}

func (r *Rule) From(c *config.RewriteRule) {
	r.SourceLabel = c.SourceLabel
	r.SkipTooShortThreshold = c.SkipTooShortThreshold
	if c.Transform != nil {
		t := &Transform{}
		if c.Transform.ToText != nil {
			toText := &ToText{
				LLM:    c.Transform.ToText.LLM,
				Prompt: c.Transform.ToText.Prompt,
			}

			toText.Type = ToTextType(c.Transform.ToText.Type)
			if toText.Type == "" {
				toText.Type = ToTextTypePrompt
			}

			t.ToText = toText
		}

		r.Transform = t
	}

	r.Match = c.Match
	if r.Match == "" {
		r.Match = c.MatchRE
	}
	r.Action = Action(c.Action)
	r.Label = c.Label
}

type Transform struct {
	ToText *ToText
}

type ToText struct {
	Type ToTextType

	// LLM is the name of the LLM to use.
	LLM string

	// Prompt is the prompt for LLM completion.
	// The source text will automatically be injected into the prompt.
	Prompt         string
	promptRendered string
}

type ToTextType string

const (
	ToTextTypePrompt ToTextType = "prompt"
	ToTextTypeScrape ToTextType = "scrape"
)

type Action string

const (
	ActionDropFeed            Action = "drop_feed"
	ActionCreateOrUpdateLabel Action = "create_or_update_label"
)

var promptTemplates = map[string]string{
	"category": `
Analyze the content and categorize it into exactly one of these categories:
Technology, Development, Entertainment, Finance, Health, Politics, Other

Classification requirements:
- Choose the SINGLE most appropriate category based on:
  * Primary topic and main focus of the content
  * Key terminology and concepts used
  * Target audience and purpose
  * Technical depth and complexity level
- For content that could fit multiple categories:
  * Identify the dominant theme
  * Consider the most specific applicable category
  * Use the primary intended purpose
- If content appears ambiguous:
  * Focus on the most prominent aspects
  * Consider the practical application
  * Choose the category that best serves user needs

Output format:
Return ONLY the category name, no other text or explanation.
Must be one of the provided categories exactly as written.
`,

	"tags": `
Analyze the content and add appropriate tags based on:
- Main topics and themes
- Key concepts and terminology 
- Target audience and purpose
- Technical depth and domain
- 2-4 tags are enough
Output format:
Return a list of tags, separated by commas, no other text or explanation.
e.g. "AI, Technology, Innovation, Future"
`,

	"score": `
Please give a score between 0 and 10 based on the following content.
Evaluate the content comprehensively considering clarity, accuracy, depth, logical structure, language expression, and completeness.
Note: If the content is an article or a text intended to be detailed, the length is an important factor. Generally, content under 300 words may receive a lower score due to lack of substance, unless its type (such as poetry or summary) is inherently suitable for brevity.
Output format:
Return the score (0-10), no other text or explanation.
E.g. "8", "5", "3", etc.
`,

	"comment_confucius": `
Please act as Confucius and write a 100-word comment on the article.
Content needs to be in line with the Chinese mainland's regulations.
Output format:
Return the comment only, no other text or explanation.
Reply short and concise, 100 words is enough.
`,

	"summary": `
Summarize the article in 100-200 words.
`,

	"summary_html_snippet": `
# Task: Create Visually Appealing Information Summary Emails

You are a professional content designer. Please convert the provided articles into **visually modern HTML email segments**, focusing on display effects in modern clients like Gmail and QQ Mail.

## Key Requirements:

1. **Output Format**:
   - Only output HTML code snippets, **no need for complete HTML document structure**
   - Only generate HTML code for a single article, so users can combine multiple pieces into a complete email
   - No explanations, additional comments, or markups
   - **No need to add titles and sources**, users will inject them automatically
   - No use html backticks, output raw html code directly
   - Output directly, no explanation, no comments, no markups

2. **Content Processing**:
   - **Don't directly copy the original text**, but extract key information and core insights from each article
   - **Each article summary should be 100-200 words**, don't force word count, adjust the word count based on the actual length of the article
   - Summarize points in relaxed, natural language, as if chatting with friends, while maintaining depth
   - Maintain the original language of the article (e.g., Chinese summary for Chinese articles)

3. **Visual Design**:
   - Design should be aesthetically pleasing with coordinated colors
   - Use sufficient whitespace and contrast
   - Maintain a consistent visual style across all articles
   - **Must use multiple visual elements** (charts, cards, quote blocks, etc.), avoid pure text presentation
   - Each article should use at least 2-3 different visual elements to make content more intuitive and readable

4. **Highlight Techniques**:

   A. **Beautiful Quote Blocks** (for highlighting important viewpoints):
   <div style="margin:20px 0; padding:20px; background:linear-gradient(to right, #f8f9fa, #ffffff); border-left:5px solid #4285f4; border-radius:5px; box-shadow:0 2px 8px rgba(0,0,0,0.05);">
     <p style="margin:0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:16px; line-height:1.6; color:#333; font-weight:500;">
       Here is the key viewpoint or finding that needs to be highlighted.
     </p>
   </div>

   B. **Information Cards** (for highlighting key data):
   <div style="display:inline-block; margin:10px 10px 10px 0; padding:15px 20px; background-color:#ffffff; border-radius:8px; box-shadow:0 3px 10px rgba(0,0,0,0.08); min-width:120px; text-align:center;">
     <p style="margin:0 0 5px 0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; color:#666;">Metric Name</p>
     <p style="margin:0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:24px; font-weight:600; color:#1a73e8;">75%</p>
   </div>

   C. **Key Points List** (for highlighting multiple points):
   <ul style="margin:20px 0; padding-left:0; list-style-type:none;">
     <li style="position:relative; margin-bottom:12px; padding-left:28px; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:15px; line-height:1.6; color:#444;">
       <span style="position:absolute; left:0; top:0; width:18px; height:18px; background-color:#4285f4; border-radius:50%; color:white; text-align:center; line-height:18px; font-size:12px;">1</span>
       First point description
     </li>
     <li style="position:relative; margin-bottom:12px; padding-left:28px; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:15px; line-height:1.6; color:#444;">
       <span style="position:absolute; left:0; top:0; width:18px; height:18px; background-color:#4285f4; border-radius:50%; color:white; text-align:center; line-height:18px; font-size:12px;">2</span>
       Second point description
     </li>
   </ul>

   D. **Emphasis Text** (for highlighting key words or phrases):
   <span style="background:linear-gradient(180deg, rgba(255,255,255,0) 50%, rgba(66,133,244,0.2) 50%); padding:0 2px;">Text to emphasize</span>

5. **Timeline Design** (suitable for event sequences or news developments):
   <div style="margin:25px 0; padding:5px 0;">
     <h3 style="font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:18px; color:#333; margin-bottom:15px;">Event Development Timeline</h3>
     
     <div style="position:relative; margin-left:30px; padding-left:30px; border-left:2px solid #e0e0e0;">
       <!-- Time Point 1 -->
       <div style="position:relative; margin-bottom:25px;">
         <div style="position:absolute; width:16px; height:16px; background-color:#4285f4; border-radius:50%; left:-40px; top:0; border:3px solid #ffffff; box-shadow:0 2px 5px rgba(0,0,0,0.1);"></div>
         <p style="margin:0 0 5px 0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; font-weight:500; color:#4285f4;">June 1, 2023</p>
         <p style="margin:0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:15px; line-height:1.5; color:#333;">Event description content, concisely explaining the key points and impact of the event.</p>
       </div>
       
       <!-- Time Point 2 -->
       <div style="position:relative; margin-bottom:25px;">
         <div style="position:absolute; width:16px; height:16px; background-color:#4285f4; border-radius:50%; left:-40px; top:0; border:3px solid #ffffff; box-shadow:0 2px 5px rgba(0,0,0,0.1);"></div>
         <p style="margin:0 0 5px 0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; font-weight:500; color:#4285f4;">June 15, 2023</p>
         <p style="margin:0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:15px; line-height:1.5; color:#333;">Event description content, concisely explaining the key points and impact of the event.</p>
       </div>
     </div>
   </div>

6. **Comparison Table** (for comparing different options or viewpoints):
   <div style="margin:25px 0; padding:15px; background-color:#f8f9fa; border-radius:8px; overflow-x:auto;">
     <table style="width:100%; border-collapse:collapse; font-family:'Google Sans',Roboto,Arial,sans-serif;">
       <thead>
         <tr>
           <th style="padding:12px 15px; text-align:left; border-bottom:2px solid #e0e0e0; color:#202124; font-weight:500;">Feature</th>
           <th style="padding:12px 15px; text-align:left; border-bottom:2px solid #e0e0e0; color:#202124; font-weight:500;">Option A</th>
           <th style="padding:12px 15px; text-align:left; border-bottom:2px solid #e0e0e0; color:#202124; font-weight:500;">Option B</th>
         </tr>
       </thead>
       <tbody>
         <tr>
           <td style="padding:12px 15px; border-bottom:1px solid #e0e0e0; color:#444;">Cost</td>
           <td style="padding:12px 15px; border-bottom:1px solid #e0e0e0; color:#444;">Higher</td>
           <td style="padding:12px 15px; border-bottom:1px solid #e0e0e0; color:#444;">Moderate</td>
         </tr>
         <tr>
           <td style="padding:12px 15px; border-bottom:1px solid #e0e0e0; color:#444;">Efficiency</td>
           <td style="padding:12px 15px; border-bottom:1px solid #e0e0e0; color:#444;">Very High</td>
           <td style="padding:12px 15px; border-bottom:1px solid #e0e0e0; color:#444;">Average</td>
         </tr>
       </tbody>
     </table>
   </div>

7. **Chart Data Processing**:
   - Bar Chart/Horizontal Bars:
   <div style="margin:20px 0; padding:15px; background-color:#f8f9fa; border-radius:8px;">
     <p style="margin:0 0 15px 0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:16px; font-weight:500; color:#333;">Data Comparison</p>
     
     <!-- Item 1 -->
     <div style="margin-bottom:12px;">
       <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:5px;">
         <span style="font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; color:#555;">Project A</span>
         <span style="font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; font-weight:500; color:#333;">65%</span>
       </div>
       <div style="height:10px; width:100%; background-color:#e8eaed; border-radius:5px; overflow:hidden;">
         <div style="height:100%; width:65%; background:linear-gradient(to right, #4285f4, #5e97f6); border-radius:5px;"></div>
       </div>
     </div>
     
     <!-- Item 2 -->
     <div style="margin-bottom:12px;">
       <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:5px;">
         <span style="font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; color:#555;">Project B</span>
         <span style="font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:14px; font-weight:500; color:#333;">42%</span>
       </div>
       <div style="height:10px; width:100%; background-color:#e8eaed; border-radius:5px; overflow:hidden;">
         <div style="height:100%; width:42%; background:linear-gradient(to right, #ea4335, #f07575); border-radius:5px;"></div>
       </div>
     </div>
   </div>

8. **Highlight Box** (for displaying tips or reminders):
   <div style="margin:25px 0; padding:20px; background-color:#fffde7; border-radius:8px; border-left:4px solid #fdd835; box-shadow:0 1px 5px rgba(0,0,0,0.05);">
     <div style="display:flex; align-items:flex-start;">
       <div style="flex-shrink:0; margin-right:15px; width:24px; height:24px; background-color:#fdd835; border-radius:50%; display:flex; align-items:center; justify-content:center;">
         <span style="color:#fff; font-weight:bold; font-size:16px;">!</span>
       </div>
       <div>
         <p style="margin:0 0 5px 0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:16px; font-weight:500; color:#333;">Tip</p>
         <p style="margin:0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:15px; line-height:1.6; color:#555;">
           Here are some additional tips or suggestions to help readers better understand or apply the article content.
         </p>
       </div>
     </div>
   </div>

9. **Summary Box**:
   <div style="margin:25px 0; padding:20px; background-color:#f2f7fd; border-radius:8px; box-shadow:0 1px 5px rgba(66,133,244,0.1);">
     <p style="margin:0 0 10px 0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:16px; font-weight:500; color:#1a73e8;">In Simple Terms</p>
     <p style="margin:0; font-family:'Google Sans',Roboto,Arial,sans-serif; font-size:15px; line-height:1.6; color:#333;">
       This is a concise summary of the entire content, highlighting the most critical findings and conclusions.
     </p>
   </div>

## Notes:
1. **Only generate content for a single article**, not including title and source, and not including HTML head and tail structure
2. Content should be **200-300 words**, don't force word count
3. **Must use multiple visual elements** (at least 2-3 types), avoid monotonous pure text presentation
4. Use relaxed, natural language, as if chatting with friends
5. Create visual charts for important data, rather than just describing with text
6. Use quote blocks to highlight important viewpoints, and lists to organize multiple points
7. Appropriately use emojis and conversational expressions to increase friendliness
8. Note that the article content has been provided in the previous message, please reply directly, no explanation, no comments, no markups
`,
}

// --- Factory code block ---

type Factory component.Factory[Rewriter, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Rewriter, config.App, Dependencies](func(instance string, app *config.App, dependencies Dependencies) (Rewriter, error) {
			m := &mockRewriter{}
			component.MockOptions(mockOn).Apply(&m.Mock)

			return m, nil
		})
	}

	return component.FactoryFunc[Rewriter, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (Rewriter, error) {
	c := &Config{}
	c.From(app)
	if err := c.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate and adjust rewrite config")
	}

	return &rewriter{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "Rewriter",
			Instance:     instance,
			Config:       c,
			Dependencies: dependencies,
		}),
	}, nil
}

// --- Implementation code block ---

type rewriter struct {
	*component.Base[Config, Dependencies]
}

func (r *rewriter) Reload(app *config.App) error {
	newConfig := &Config{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate and adjust rewrite config")
	}
	r.SetConfig(newConfig)

	return nil
}

func (r *rewriter) Labels(ctx context.Context, labels model.Labels) (rewritten model.Labels, err error) {
	ctx = telemetry.StartWith(ctx, append(r.TelemetryLabels(), telemetrymodel.KeyOperation, "Labels")...)
	defer func() { telemetry.End(ctx, err) }()

	rules := *r.Config()
	for _, rule := range rules {
		// Get source text based on source label.
		sourceText := labels.Get(rule.SourceLabel)
		if utf8.RuneCountInString(sourceText) < *rule.SkipTooShortThreshold {
			continue
		}

		// Transform text if configured.
		text := sourceText
		if rule.Transform != nil {
			transformed, err := r.transformText(ctx, rule.Transform, sourceText)
			if err != nil {
				return nil, errors.Wrap(err, "transform text")
			}
			text = transformed
		}

		// Check if text matches the rule.
		if !rule.matchRE.MatchString(text) {
			continue
		}

		// Handle actions.
		switch rule.Action {
		case ActionDropFeed:
			return nil, nil
		case ActionCreateOrUpdateLabel:
			labels.Put(rule.Label, text, false)
		}
	}

	labels.EnsureSorted()

	return labels, nil
}

// transformText transforms text using configured LLM.
func (r *rewriter) transformText(ctx context.Context, transform *Transform, text string) (string, error) {
	switch transform.ToText.Type {
	case ToTextTypeScrape:
		return r.transformTextScrape(ctx, text)
	case ToTextTypePrompt:
		return r.transformTextPrompt(ctx, transform, text)
	default:
		return r.transformTextPrompt(ctx, transform, text)
	}
}

func (r *rewriter) transformTextScrape(ctx context.Context, text string) (string, error) {
	url := text

	md, err := crawlutil.Markdown(ctx, url)
	if err != nil {
		return "", errors.Wrapf(err, "scrape %s", url)
	}

	return md, nil
}

func (r *rewriter) transformTextPrompt(ctx context.Context, transform *Transform, text string) (string, error) {
	// Get LLM instance.
	llm := r.Dependencies().LLMFactory.Get(transform.ToText.LLM)

	// Call completion.
	result, err := llm.String(ctx, []string{
		transform.ToText.promptRendered,
		"The content to be processed is below, and the processing requirements are as above",
		text, // TODO: may place to first line to hit the model cache in different rewrite rules.
	})
	if err != nil {
		return "", errors.Wrap(err, "llm completion")
	}

	return r.transformTextHack(result), nil
}

func (r *rewriter) transformTextHack(text string) string {
	bytes := unsafe.Slice(unsafe.StringData(text), len(text))
	start := 0
	end := len(bytes)

	// Remove the last line if it's empty.
	// This is a hack to avoid the model output a empty line.
	// E.g. category: tech\n
	if end > 0 && bytes[end-1] == '\n' {
		end--
	}

	// Remove the html backticks.
	if end-start >= 7 && string(bytes[start:start+7]) == "```html" {
		start += 7
	}
	if end-start >= 3 && string(bytes[end-3:end]) == "```" {
		end -= 3
	}

	// If no changes, return the original string.
	if start == 0 && end == len(bytes) {
		return text
	}

	// Only copy one time.
	return string(bytes[start:end])
}

type mockRewriter struct {
	component.Mock
}

func (r *mockRewriter) Reload(app *config.App) error {
	args := r.Called(app)

	return args.Error(0)
}

func (r *mockRewriter) Labels(ctx context.Context, labels model.Labels) (model.Labels, error) {
	args := r.Called(ctx, labels)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(model.Labels), args.Error(1)
}
