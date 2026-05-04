package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

const websiteAgentPrompt = `Role: You are a highly accurate AI assistant specializing in crafting well-structured, visually appealing, and modern websites. Your creations should be user-friendly, responsive by default, and incorporate best practices for web design.

Objective: Generate a website spec or exactly one requested website file based on the provided brand concept.

The website should be ready for initial review and deployment, with clear placeholders where specific user content or images are required.

Follow the output contract in each user prompt. Some prompts require JSON. File-generation prompts require raw file contents only.`

const websiteCreationGuidance = `Website creation guidance adapted from the reference website creation agent:

Role and objective:
- Craft well-structured, visually appealing, modern websites that are user-friendly, responsive by default, and follow web design best practices.
- Generate complete HTML, CSS, and any necessary basic JavaScript for a foundational marketing website based on the provided topic or brand concept.
- Make the website ready for initial review and deployment, with clear placeholders where specific user content or images are required.

Input handling:
- Use the provided domain name, brand or project name, primary goal, key services/products/information, target audience, and any visual direction.
- If details are missing, use sensible defaults and clear placeholders instead of stopping.

Planning guidance:
- Default to a multi-page structure unless the concept is exceptionally simple.
- Typical core pages include a homepage, an about page, a services/products/offerings page, and a contact page, but choose the file list that best fits the brand.
- Ensure clear navigation among generated pages.

Design and layout principles:
- Aim for a clean, modern, professional design that reflects any style hints in the input.
- The website must be fully responsive on desktop, tablet, and mobile.
- Use CSS Flexbox and/or Grid for layout.
- Choose legible web-safe fonts.
- Select a harmonious and accessible color palette using inferable brand colors or tasteful defaults.

Content and sections:
- Use a consistent header with the brand name or logo placeholder and navigation links.
- Use a consistent footer with a copyright notice and optional placeholder links.
- A homepage usually benefits from a hero section, clear headline, sub-headline, CTA, key offerings, about teaser, and optional testimonials.
- An about-style page usually includes mission, values, history, and optional team placeholders.
- A services/products/offerings page usually uses structured cards or lists with headings, descriptions, and image placeholders.
- A contact page usually includes contact information placeholders and a static client-side contact form.

Placeholders and images:
- Use relevant, descriptive placeholder text. Lorem Ipsum is acceptable for long body copy, but short copy should be contextual to the brand.
- Clearly mark where users should insert their own text, images, or company-specific details.
- Use image placeholders where appropriate, such as https://via.placeholder.com/800x600.png?text=Relevant+Image.

Code quality:
- Generate semantic HTML5.
- Use a responsive viewport meta tag in HTML files.
- Put shared styles in an external CSS file when the spec includes one, and link it from HTML files.
- Put JavaScript in an external JS file when the spec includes one, and keep it minimal and unobtrusive.
- Do not use external packages.
- Do not generate server-side code, backend form handling, database setup, binary assets, or image files.`

type websiteSpec struct {
	Brief string               `json:"brief,omitempty"`
	Files []websitePlannedFile `json:"files"`
}

type websitePlannedFile struct {
	Filename string `json:"filename"`
	Purpose  string `json:"purpose,omitempty"`
}

type generatedWebsiteFile struct {
	Filename string
	Content  string
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
	spec, err := a.createWebsiteSpec(ctx, websiteContext)
	if err != nil {
		return WebsiteContent{}, err
	}

	files := make(map[string]string, len(spec.Files))
	generatedFiles := make([]generatedWebsiteFile, 0, len(spec.Files))
	for _, plannedFile := range spec.Files {
		content, err := a.generateWebsiteFile(ctx, plannedFile, websiteContext, spec, generatedFiles)
		if err != nil {
			return WebsiteContent{}, err
		}
		files[plannedFile.Filename] = content
		generatedFiles = append(generatedFiles, generatedWebsiteFile{
			Filename: plannedFile.Filename,
			Content:  content,
		})
	}

	return WebsiteContent{Files: files}, nil
}

func (a *WebsiteAgentImpl) createWebsiteSpec(ctx context.Context, websiteContext string) (websiteSpec, error) {
	resp, err := a.agent.LLMCall(ctx, buildWebsiteSpecPrompt(websiteContext))
	if err != nil {
		return websiteSpec{}, fmt.Errorf("create website spec: llm call: %w", err)
	}

	var payload websiteSpec
	if !unmarshalJSONFromLLMResponse(resp, &payload) {
		return websiteSpec{}, fmt.Errorf("create website spec: response was not valid JSON")
	}
	if err := validateWebsiteSpec(payload); err != nil {
		return websiteSpec{}, fmt.Errorf("create website spec: %w", err)
	}

	return payload, nil
}

func (a *WebsiteAgentImpl) generateWebsiteFile(ctx context.Context, plannedFile websitePlannedFile, websiteContext string, spec websiteSpec, generatedFiles []generatedWebsiteFile) (string, error) {
	prompt, err := buildWebsiteFilePrompt(plannedFile, websiteContext, spec, generatedFiles)
	if err != nil {
		return "", fmt.Errorf("generate website file %s: build prompt: %w", plannedFile.Filename, err)
	}

	resp, err := a.agent.LLMCall(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate website file %s: llm call: %w", plannedFile.Filename, err)
	}

	content := strings.TrimSpace(resp)
	if content == "" {
		return "", fmt.Errorf("generate website file %s: response content was empty", plannedFile.Filename)
	}

	return content, nil
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

func buildWebsiteSpecPrompt(websiteContext string) string {
	return fmt.Sprintf(`Create a general website spec and file list for a foundational marketing website.

Brand context:
%s

%s

Instructions:
- Use the guidance above to choose the website structure, visual direction, content approach, and file list.
- Use sensible defaults and placeholders where specific business content is missing.
- Include only text files that can be generated directly as HTML, CSS, or basic JavaScript.
- Choose filenames and generation order intentionally. Each later file will receive all previously generated files as context.
- Keep the file set practical for initial review and deployment.
- Do not include server-side code, backend form handling, database setup, binary assets, or image files.

Output Requirements:
Return valid JSON only:
{
  "brief": "Site-specific design, content, structure, and implementation notes for later file generation.",
  "files": [
    {
      "filename": "index.html",
      "purpose": "What this file should contain and how it supports the complete website."
    }
  ]
}

Rules:
- Return only the JSON object.
- Do not include markdown fences.
- Do not include explanations outside JSON.
- The brief should be specific to the provided brand and useful to subsequent file-generation calls.
- Include a purpose for each file when useful.`,
		websiteContext,
		websiteCreationGuidance,
	)
}

func buildWebsiteFilePrompt(plannedFile websitePlannedFile, websiteContext string, spec websiteSpec, generatedFiles []generatedWebsiteFile) (string, error) {
	specJSON, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", err
	}
	purpose := strings.TrimSpace(plannedFile.Purpose)
	if purpose == "" {
		purpose = "No explicit purpose was provided. Infer it from the filename, brand context, website spec, and previously generated files."
	}

	return fmt.Sprintf(`Generate exactly one file for this website.

Requested file:
%s

Requested file purpose:
%s

Brand context:
%s

%s

Website spec:
%s

Previously generated files:
%s

Instructions:
- Generate only %s.
- Use the website spec and previously generated files as the source of truth for naming, links, CSS classes, shared layout, and consistency.
- For HTML files, generate semantic HTML5, include a responsive viewport meta tag, include consistent navigation where appropriate, and include comments/placeholders where users should insert specific content or images.
- For CSS files, generate clean, organized responsive CSS for the actual HTML/classes in the spec and previously generated files.
- For JavaScript files, generate minimal unobtrusive JavaScript only. Do not use external packages.
- Do not generate server-side code, backend form handling, database setup, or binary assets.

Output Requirements:
- Return the raw contents of %s only.
- Do not return JSON.
- Do not include markdown fences.
- Do not include explanations.
- Do not include any other file.`,
		plannedFile.Filename,
		purpose,
		websiteContext,
		websiteCreationGuidance,
		string(specJSON),
		formatGeneratedFiles(generatedFiles),
		plannedFile.Filename,
		plannedFile.Filename,
	), nil
}

func validateWebsiteSpec(spec websiteSpec) error {
	if len(spec.Files) == 0 {
		return fmt.Errorf("file list was empty")
	}

	for _, file := range spec.Files {
		filename := strings.TrimSpace(file.Filename)
		if filename == "" {
			return fmt.Errorf("planned filename was empty")
		}
	}

	return nil
}

func formatGeneratedFiles(files []generatedWebsiteFile) string {
	if len(files) == 0 {
		return "No files have been generated yet."
	}

	var b strings.Builder
	for _, file := range files {
		b.WriteString("\n--- BEGIN ")
		b.WriteString(file.Filename)
		b.WriteString(" ---\n")
		b.WriteString(file.Content)
		b.WriteString("\n--- END ")
		b.WriteString(file.Filename)
		b.WriteString(" ---\n")
	}
	return b.String()
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
