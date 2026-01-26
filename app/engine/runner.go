package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/oliveagle/jsonpath"
)

type RunnerSettings struct {
	// The settings file for this project.
	Cfg             *config.Cfg
	BaseUrlOverride string
}

func (self *RunnerSettings) getBaseUrl() string {
	if self.BaseUrlOverride != "" {
		return self.BaseUrlOverride
	}

	return self.Cfg.BaseUrl
}

type Runner struct {
	settings RunnerSettings
	RunId    string
	stepsRan int
	// cookieJar is a per-run cookie jar to maintain stateful cookies across steps.
	//
	// it implicitly stores and sends cookies for each request.
	cookieJar *cookiejar.Jar
	symtable  map[string]any
}

func NewRunner(settings RunnerSettings) *Runner {
	runId := utils.NewId()

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create cookie jar: %v", err))
	}

	return &Runner{
		settings:  settings,
		RunId:     runId,
		stepsRan:  0,
		cookieJar: cookieJar,
		symtable:  map[string]any{},
	}
}

// Execute a step.
//
// Returns an AssertionFailure on an error caused from assertion failure which is not an actual error.
func (self *Runner) Execute(step *app.Step) ([]byte, error) {
	baseCtx := context.Background()
	timeout := 30 * time.Second // have a default
	if step.Options.Timeout.IsSome() {
		var err error
		timeout, err = utils.ToDuration(step.Options.Timeout.Unwrap())
		if err != nil {
			return []byte{}, oops.Err(oops.StepRequestProcessingFailed, "failed to parse timeout duration for step", err)
		}
	}

	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Setup request body

	self.processBindingsForStep(step)

	var body io.Reader
	var contentType string

	if step.Request.Json.IsSome() {
		payload := step.Request.Json.Unwrap()
		b, err := json.Marshal(payload)
		if err != nil {
			return []byte{}, oops.Err(oops.StepRequestBuildFailed, "failed to marshal JSON body", err)
		}
		body = bytes.NewReader(b)
		contentType = "application/json"
	} else if step.Request.Xml.IsSome() {
		xmlData := step.Request.Xml.Unwrap()
		body = bytes.NewReader([]byte(xmlData))
		contentType = "application/xml"
	} else if step.Request.Files.IsSome() {
		// Handle file uploads with multipart/form-data
		files := step.Request.Files.Unwrap()

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		configDir := self.settings.Cfg.GetConfigDir()
		if configDir == "" {
			return []byte{}, oops.Err(oops.Internal, "config directory not available for resolving file paths", nil)
		}

		for fieldName, relativePath := range files {
			// Resolve relative path to absolute path
			absolutePath := filepath.Join(configDir, relativePath)

			// Check file size (100MB limit)
			const maxFileSize = 100 * 1024 * 1024 // 100MB
			fileInfo, err := os.Stat(absolutePath)
			if err != nil {
				if os.IsNotExist(err) {
					return []byte{}, oops.Err(oops.FileNotFound, fmt.Sprintf("file not found: %s", relativePath), err)
				}
				return []byte{}, oops.Err(oops.FileReadError, fmt.Sprintf("failed to stat file: %s", relativePath), err)
			}

			if fileInfo.Size() > maxFileSize {
				return []byte{}, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("file too large: %s (%.2f MB, max 100MB)", relativePath, float64(fileInfo.Size())/(1024*1024)), nil)
			}

			// Open file
			file, err := os.Open(absolutePath)
			if err != nil {
				return []byte{}, oops.Err(oops.FileReadError, fmt.Sprintf("failed to open file: %s", relativePath), err)
			}
			defer file.Close()

			// Auto-detect MIME type from file extension (like curl does)
			mimeType := mime.TypeByExtension(filepath.Ext(absolutePath))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			// Create form file
			part, err := writer.CreateFormFile(fieldName, filepath.Base(absolutePath))
			if err != nil {
				return []byte{}, oops.Err(oops.StepRequestBuildFailed, "failed to create multipart form file", err)
			}

			// Copy file content to part
			_, err = io.Copy(part, file)
			if err != nil {
				return []byte{}, oops.Err(oops.StepRequestBuildFailed, "failed to copy file content", err)
			}
		}

		// Close the writer to finalize the multipart message
		err := writer.Close()
		if err != nil {
			return []byte{}, oops.Err(oops.StepRequestBuildFailed, "failed to close multipart writer", err)
		}

		body = &buf
		contentType = writer.FormDataContentType()
	}

	url := fmt.Sprintf("%s%s", self.settings.getBaseUrl(), step.Request.Path)
	method := strings.ToUpper(step.Request.Method)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return []byte{}, oops.Err(oops.Internal, "failed to initialize the request for the step", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Apply custom headers (these can override auto-headers like Content-Type)
	if step.Request.Headers.IsSome() {
		headers := step.Request.Headers.Unwrap()
		for headerName, headerValue := range headers {
			req.Header.Set(headerName, headerValue)
		}
	}

	// Send the request

	client := http.DefaultClient
	if !step.Request.DisableHeaders {
		client = &http.Client{
			Jar: self.cookieJar,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return []byte{}, &AssertionFailure{
				Err:      oops.Err(oops.StepRequestDeadlineExceeded, "request was cancelled by context deadline", err),
				Response: nil,
				Step:     step,
			}
		} else {
			return []byte{}, oops.Err(oops.StepRequestFailed, "failed to execute the request for the step", err)
		}
	}
	defer resp.Body.Close()

	self.stepsRan += 1

	responseBodyInBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, oops.Err(oops.StepResponseReadFailed, "failed to read response body", err)
	}

	// Validate assertion

	responseContentType := resp.Header.Get("Content-Type")
	err = validateAssertClause(&step.Assert, resp.StatusCode, responseBodyInBytes, responseContentType)
	if err != nil {
		return []byte{}, &AssertionFailure{
			Err:      oops.Err(oops.StepRequestAssertionFailed, "step request assertion failed", err),
			Response: responseBodyInBytes,
			Step:     step,
		}
	}

	// Set exports if they exist

	if len(step.Exports) != 0 {
		err := self.processExports(step.Exports, responseBodyInBytes, responseContentType)
		if err != nil {
			return []byte{}, err
		}
	}

	return responseBodyInBytes, nil
}

func (self *Runner) StepsRan() int {
	return self.stepsRan
}

func (self *Runner) TotalSteps() int {
	return self.settings.Cfg.GetTotalSteps()
}

func (self *Runner) processBindingsForStep(step *app.Step) error {
	// process url path since that might be injected for a dynamic route
	step.Request.Path = self.resolveBindingFromString(step.Request.Path)

	// Process body
	if step.Request.Json.IsSome() {
		requestBody := step.Request.Json.Unwrap()
		processedRequestBody, err := self.processRequestBody(requestBody)
		if err != nil {
			return oops.Err(oops.StepRequestProcessingFailed, "failed to process request body", err)
		}
		step.Request.Json = Some(processedRequestBody)
	} else if step.Request.Xml.IsSome() {
		xmlData := step.Request.Xml.Unwrap()
		step.Request.Xml = Some(self.resolveBindingFromString(xmlData))
	}

	// Process assertions
	if step.Assert.All.IsSome() {
		assertions := step.Assert.All.Unwrap()
		for _, assertion := range assertions {
			if assertion.Contains.IsSome() {
				contains := assertion.Contains.Unwrap()
				assertion.Contains = Some(self.resolveBindingFromString(contains))
			}
			if assertion.Equals.IsSome() {
				equals := assertion.Equals.Unwrap()
				assertion.Equals = Some(self.resolveBindingFromString(equals))
			}
		}
	}

	return nil
}

// processRequestBody takes the request body and processes them by replacing any injectable variable (like {{RUN_ID}}) with its value.
func (self *Runner) processRequestBody(body map[string]any) (map[string]any, error) {
	var err error

	for k, v := range body {
		switch val := v.(type) {
		case string:
			body[k] = self.resolveBindingFromString(val)
		case map[string]any:
			body[k], err = self.processRequestBody(val)
			if err != nil {
				return nil, oops.Err(oops.StepRequestProcessingFailed, "failed to process/modify request body", err)
			}
		// optionally handle []any if you expect arrays
		case []any:
			for i, elem := range val {
				if s, ok := elem.(string); ok {
					val[i] = self.resolveBindingFromString(s)
				}
			}
			body[k] = val
		}
	}

	return body, nil
}

// resolveBindingFromString takes a string and replaces any injectable bindings built-in or defined
//
// Example 1: "test-{{RUN_ID}}" -> "test-12345"
//
// Example 2: "test-{{RUN_ID}}" -> "test-12345"
func (self *Runner) resolveBindingFromString(s string) string {
	reBinding := regexp.MustCompile(`\{\{([a-zA-Z0-9_:-]+)\}\}`)

	return reBinding.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2] // strip {{ }}

		// built-ins
		resolved, ok := self.resolveBuiltinBinding(inner)
		if ok {
			return resolved
		}

		// user bindings: bind:key
		if strings.HasPrefix(inner, "bind:") {
			key := inner[5:]
			if v, ok := self.symtable[key]; ok {
				return fmt.Sprintf("%v", v)
			}
			return m // unresolved stays as-is
		}

		return m
	})
}

func (self *Runner) processExports(exports app.Exports, responseBody []byte, contentType string) error {
	isXML := strings.Contains(strings.ToLower(contentType), "xml")

	if isXML {
		return self.processXMLExports(exports, responseBody)
	}
	return self.processJSONExports(exports, responseBody)
}

func (self *Runner) processJSONExports(exports app.Exports, responseBody []byte) error {
	var body map[string]any
	err := json.Unmarshal(responseBody, &body)
	if err != nil {
		return oops.Err(oops.StepResponseReadFailed, "failed to unmarshal response body for exports", err)
	}

	for ident, jspath := range exports {
		value, err := jsonpath.JsonPathLookup(body, jspath)
		if err != nil {
			return oops.Err(oops.StepExportFailed, fmt.Sprintf("failed to lookup jsonpath %s for export %s", jspath, ident), err)
		}
		self.symtable[ident] = value
	}
	return nil
}

func (self *Runner) processXMLExports(exports app.Exports, responseBody []byte) error {
	doc, err := xmlquery.Parse(bytes.NewReader(responseBody))
	if err != nil {
		return oops.Err(oops.StepResponseReadFailed, "failed to parse XML response body for exports", err)
	}

	for ident, xpath := range exports {
		node := xmlquery.FindOne(doc, xpath)
		if node == nil {
			return oops.Err(oops.StepExportFailed, fmt.Sprintf("failed to lookup xpath %s for export %s", xpath, ident), nil)
		}
		self.symtable[ident] = node.InnerText()
	}
	return nil
}

func (self *Runner) resolveBuiltinBinding(str string) (string, bool) {
	switch str {
	case "RUN_ID":
		return self.RunId, true
	case "RAND_DIGIT":
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic("rng unavailable: veriflow requires OS entropy")
		}
		return fmt.Sprintf("%d", n.Int64()), true
	}

	return "", false
}
