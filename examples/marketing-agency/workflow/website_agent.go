package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vaastav/dmas_forge/ai_runtime/core"
)

const websiteAgentPrompt = `Role: You are a highly accurate AI assistant specializing in crafting well-structured, visually appealing, and modern websites. Your creations should be user-friendly, responsive by default, and incorporate best practices for web design.

Objective: Generate complete HTML, CSS, or basic JavaScript code for exactly one requested file in a foundational, multi-page marketing website based on the provided brand concept.

The website should be ready for initial review and deployment, with clear placeholders where specific user content or images are required.

Always return valid JSON only. Do not wrap output in markdown.`

type websiteFilePlan struct {
	Files []websitePlannedFile `json:"files"`
}

type websitePlannedFile struct {
	Filename string `json:"filename"`
	Purpose  string `json:"purpose"`
}

type websiteFileResponse struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type WebsiteAgentImpl struct {
	agent core.Agent
}

func NewWebsiteAgentImpl(ctx context.Context, agent core.Agent) (WebsiteAgent, error) {
	a := &WebsiteAgentImpl{agent: agent}
	if err := a.agent.AddSystemPrompt(ctx, websiteAgentPrompt); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *WebsiteAgentImpl) GenerateWebsite(ctx context.Context, domain string, brandInfo BrandInfo) (WebsiteContent, error) {
	websiteContext := buildWebsiteContext(domain, brandInfo)
	filePlan, err := a.createWebsiteFilePlan(ctx, websiteContext)
	if err != nil {
		return WebsiteContent{}, err
	}

	files := make(map[string]string, len(filePlan.Files))
	for _, plannedFile := range filePlan.Files {
		content, err := a.generateWebsiteFile(ctx, plannedFile, websiteContext, filePlan)
		if err != nil {
			return WebsiteContent{}, err
		}
		files[plannedFile.Filename] = content
	}

	return WebsiteContent{Files: files}, nil
}

func (a *WebsiteAgentImpl) createWebsiteFilePlan(ctx context.Context, websiteContext string) (websiteFilePlan, error) {
	resp, err := a.agent.LLMCall(ctx, buildWebsitePlanPrompt(websiteContext))
	if err != nil {
		return websiteFilePlan{}, fmt.Errorf("create website file plan: llm call: %w", err)
	}

	var payload websiteFilePlan
	if !unmarshalJSONFromLLMResponse(resp, &payload) {
		return websiteFilePlan{}, fmt.Errorf("create website file plan: response was not valid JSON")
	}
	if err := validateWebsitePlan(payload); err != nil {
		return websiteFilePlan{}, fmt.Errorf("create website file plan: %w", err)
	}

	return payload, nil
}

func (a *WebsiteAgentImpl) generateWebsiteFile(ctx context.Context, plannedFile websitePlannedFile, websiteContext string, filePlan websiteFilePlan) (string, error) {
	prompt, err := buildWebsiteFilePrompt(plannedFile, websiteContext, filePlan)
	if err != nil {
		return "", fmt.Errorf("generate website file %s: build prompt: %w", plannedFile.Filename, err)
	}

	resp, err := a.agent.LLMCall(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate website file %s: llm call: %w", plannedFile.Filename, err)
	}

	var payload websiteFileResponse
	if !unmarshalJSONFromLLMResponse(resp, &payload) {
		return "", fmt.Errorf("generate website file %s: response was not valid JSON", plannedFile.Filename)
	}
	if payload.Filename != plannedFile.Filename {
		return "", fmt.Errorf("generate website file %s: response filename %q did not match requested file", plannedFile.Filename, payload.Filename)
	}
	if strings.TrimSpace(payload.Content) == "" {
		return "", fmt.Errorf("generate website file %s: response content was empty", plannedFile.Filename)
	}

	return payload.Content, nil
}

func buildWebsiteContext(domain string, brandInfo BrandInfo) string {
	domain = defaultString(domain, "yourdomain.com")
	brandName := defaultString(brandInfo.Name, "Your Brand Name Here")
	description := defaultString(
		brandInfo.Description,
		"A professional marketing website that presents the brand, explains its offerings, and encourages visitors to make contact.",
	)
	keywords := trimNonEmpty(brandInfo.Keywords)
	if len(keywords) == 0 {
		keywords = []string{"services", "products", "offerings"}
	}
	targetAudience := defaultString(brandInfo.TargetAudience, "Prospective customers interested in the brand's services or products")

	return fmt.Sprintf(`Domain Name: %s
Brand/Project Name: %s
Primary Goal/Purpose of the Website: %s
Key Services, Products, or Information to be Featured: %s
Target Audience Description: %s

Shared Design Direction:
Clean, modern, professional, responsive, accessible, and aligned with the brand concept.`,
		domain,
		brandName,
		description,
		strings.Join(keywords, ", "),
		targetAudience,
	)
}

func buildWebsitePlanPrompt(websiteContext string) string {
	return fmt.Sprintf(`Decide the website shape and file list for a foundational marketing website.

Brand context:
%s

Instructions:
- Choose a concise multi-page website structure that fits the brand.
- Use sensible defaults and placeholders where specific business content is missing.
- Include only text files that can be generated directly as HTML, CSS, or JavaScript.
- Include an index.html homepage.
- Include shared CSS.
- Include JavaScript only if useful for minimal client-side enhancement.
- Keep the file set small enough for initial review and deployment.
- Use simple relative filenames only. Do not include directories or absolute paths.
- Allowed file extensions: .html, .css, .js.
- Do not include server-side code, backend form handling, database setup, binary assets, or image files.

Output Requirements:
Return valid JSON only:
{
  "files": [
    {
      "filename": "index.html",
      "purpose": "Homepage with hero, clear value proposition, navigation, and primary call to action."
    }
  ]
}

Rules:
- Return only the JSON object.
- Do not include markdown fences.
- Do not include explanations outside JSON.
- Each purpose must be a concise explanation of what that file should contain and how it supports the complete website.`,
		websiteContext,
	)
}

func buildWebsiteFilePrompt(plannedFile websitePlannedFile, websiteContext string, filePlan websiteFilePlan) (string, error) {
	planJSON, err := json.MarshalIndent(filePlan, "", "  ")
	if err != nil {
		return "", err
	}
	filenames := plannedFilenames(filePlan)

	return fmt.Sprintf(`Generate exactly one file for this website.

Requested file:
%s

Requested file purpose:
%s

Brand context:
%s

Website plan:
%s

Instructions:

Understand the Core Need:
- Analyze the brand concept, primary goal, key offerings, and audience.
- Use sensible placeholders where specific business content is missing.

Plan Website Structure & Pages:
- The complete website consists only of the files listed in the website plan.
- Ensure clear navigation among the planned HTML pages.
- Generate only the requested file.

Design & Layout Principles:
- Aim for a clean, modern, professional design.
- The website must be fully responsive on desktop, tablet, and mobile.
- Use CSS Flexbox and/or Grid for layout.
- Choose legible web-safe fonts.
- Use a harmonious and accessible color palette.

Develop Page Content & Sections:
- Header: Include the brand name or logo placeholder and navigation links to all main pages.
- Footer: Include a copyright notice and optional placeholder links.
- index.html: Include a hero section, compelling headline, sub-headline, CTA, key offerings, about teaser, and optional testimonials.
- Any about-style page: Include mission, values, history, and optional team placeholders.
- Any services, products, or offerings page: Include structured offerings with headings, descriptions, and image placeholders.
- Any contact page: Include contact information placeholders and a simple client-side contact form.
- Any CSS file: Include all shared styles for layout, typography, header, navigation, hero, cards/lists, forms, footer, and responsive behavior.
- Any JavaScript file: Include only minimal unobtrusive JavaScript, such as mobile navigation behavior or simple progressive enhancements.

Content Placeholders & Images:
- Use relevant and descriptive placeholder text.
- Clearly mark where users should insert their own content.
- Use image placeholders where appropriate.
- Use placeholder services only when needed, such as https://via.placeholder.com/800x600.png?text=Relevant+Image.

Code Quality & Files:
- For HTML files, generate semantic HTML5, include <!DOCTYPE html>, include a responsive viewport meta tag, link the planned CSS file in the head, link the planned JavaScript file before the closing body tag when the plan includes one, and include comments guiding users where to insert specific content or images.
- If the plan has no JavaScript file, do not include a script tag.
- For CSS files, generate clean, well-organized CSS comments and responsive styles for all planned pages.
- For JavaScript files, generate minimal client-side JavaScript only. Do not use external packages.
- Do not generate server-side code, backend form handling, or database setup.
- All internal links and asset references must use these planned filenames exactly: %s.

Output Requirements:
Return valid JSON only:
{
  "filename": "%s",
  "content": "..."
}

Rules:
- Generate only %s.
- Do not include markdown fences.
- Do not include explanations.
- Do not include any other file.
- The content field must contain the complete contents of %s.`,
		plannedFile.Filename,
		plannedFile.Purpose,
		websiteContext,
		string(planJSON),
		strings.Join(filenames, ", "),
		plannedFile.Filename,
		plannedFile.Filename,
		plannedFile.Filename,
	), nil
}

func validateWebsitePlan(plan websiteFilePlan) error {
	if len(plan.Files) == 0 {
		return fmt.Errorf("file list was empty")
	}

	seen := map[string]bool{}
	hasIndex := false
	hasCSS := false
	for _, file := range plan.Files {
		filename := strings.TrimSpace(file.Filename)
		purpose := strings.TrimSpace(file.Purpose)
		if filename == "" {
			return fmt.Errorf("planned filename was empty")
		}
		if purpose == "" {
			return fmt.Errorf("planned purpose for %s was empty", filename)
		}
		if filename != file.Filename {
			return fmt.Errorf("planned filename %q had surrounding whitespace", file.Filename)
		}
		if filename != filepath.Base(filename) || strings.Contains(filename, `\`) {
			return fmt.Errorf("planned filename %q must be a simple relative filename", filename)
		}
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".html":
			if filename == "index.html" {
				hasIndex = true
			}
		case ".css":
			hasCSS = true
		case ".js":
		default:
			return fmt.Errorf("planned filename %q used unsupported extension %q", filename, ext)
		}
		if seen[filename] {
			return fmt.Errorf("planned filename %q was duplicated", filename)
		}
		seen[filename] = true
	}
	if !hasIndex {
		return fmt.Errorf("file list did not include index.html")
	}
	if !hasCSS {
		return fmt.Errorf("file list did not include a CSS file")
	}

	return nil
}

func plannedFilenames(plan websiteFilePlan) []string {
	filenames := make([]string, 0, len(plan.Files))
	for _, file := range plan.Files {
		filenames = append(filenames, file.Filename)
	}
	return filenames
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func trimNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
