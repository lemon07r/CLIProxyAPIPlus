// Package gemini provides request translation functionality for Gemini CLI to Gemini API compatibility.
// It handles parsing and transforming Gemini CLI API requests into Gemini API format,
// extracting model information, system instructions, message contents, and tool declarations.
// The package performs JSON data transformation to ensure compatibility
// between Gemini CLI API format and Gemini API's expected format.
package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/common"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func generateToolID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "toolu_" + hex.EncodeToString(b)
}

// ConvertGeminiRequestToAntigravity parses and transforms a Gemini CLI API request into Gemini API format.
// It extracts the model name, system instruction, message contents, and tool declarations
// from the raw JSON request and returns them in the format expected by the Gemini API.
// The function performs the following transformations:
// 1. Extracts the model information from the request
// 2. Restructures the JSON to match Gemini API format
// 3. Converts system instructions to the expected format
// 4. Fixes CLI tool response format and grouping
//
// Parameters:
//   - modelName: The name of the model to use for the request (unused in current implementation)
//   - rawJSON: The raw JSON request data from the Gemini CLI API
//   - stream: A boolean indicating if the request is for a streaming response (unused in current implementation)
//
// Returns:
//   - []byte: The transformed request data in Gemini API format
func ConvertGeminiRequestToAntigravity(modelName string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := inputRawJSON
	template := ""
	template = `{"project":"","request":{},"model":""}`
	template, _ = sjson.SetRaw(template, "request", string(rawJSON))
	template, _ = sjson.Set(template, "model", modelName)
	template, _ = sjson.Delete(template, "request.model")

	template, errFixCLIToolResponse := fixCLIToolResponse(template)
	if errFixCLIToolResponse != nil {
		return []byte{}
	}

	systemInstructionResult := gjson.Get(template, "request.system_instruction")
	if systemInstructionResult.Exists() {
		template, _ = sjson.SetRaw(template, "request.systemInstruction", systemInstructionResult.Raw)
		template, _ = sjson.Delete(template, "request.system_instruction")
	}
	rawJSON = []byte(template)

	// Normalize roles in request.contents: default to valid values if missing/invalid
	contents := gjson.GetBytes(rawJSON, "request.contents")
	if contents.Exists() {
		prevRole := ""
		idx := 0
		contents.ForEach(func(_ gjson.Result, value gjson.Result) bool {
			role := value.Get("role").String()
			valid := role == "user" || role == "model"
			if role == "" || !valid {
				var newRole string
				if prevRole == "" {
					newRole = "user"
				} else if prevRole == "user" {
					newRole = "model"
				} else {
					newRole = "user"
				}
				path := fmt.Sprintf("request.contents.%d.role", idx)
				rawJSON, _ = sjson.SetBytes(rawJSON, path, newRole)
				role = newRole
			}
			prevRole = role
			idx++
			return true
		})
	}

	// Handle assistant prefill for Claude models routed via the Gemini translator:
	// if the last content has role "model", the Antigravity API will reject the request.
	// Only applies to Claude models — native Gemini models do not receive prefill.
	if strings.Contains(modelName, "claude") {
		contents = gjson.GetBytes(rawJSON, "request.contents")
		if contents.Exists() && contents.IsArray() {
			contentsArray := contents.Array()
			if len(contentsArray) > 0 {
				lastContent := contentsArray[len(contentsArray)-1]
				if lastContent.Get("role").String() == "model" {
					// Check that the last model message has no functionCall parts
					// (those are legitimate mid-conversation tool-use turns, not prefill)
					hasFunctionCall := false
					lastContent.Get("parts").ForEach(func(_, part gjson.Result) bool {
						if part.Get("functionCall").Exists() {
							hasFunctionCall = true
							return false
						}
						return true
					})

					if !hasFunctionCall {
						// Collect text parts from the trailing model message
						var prefillTexts []string
						lastContent.Get("parts").ForEach(func(_, part gjson.Result) bool {
							if t := part.Get("text").String(); t != "" {
								prefillTexts = append(prefillTexts, t)
							}
							return true
						})

						// Remove the trailing model message
						lastIdx := len(contentsArray) - 1
						rawJSON, _ = sjson.DeleteBytes(rawJSON, fmt.Sprintf("request.contents.%d", lastIdx))

						// If the prefill had text content, inject a synthetic user message
						if len(prefillTexts) > 0 {
							prefill := strings.Join(prefillTexts, "")
							syntheticUser := `{"role":"user","parts":[]}`
							partJSON := `{}`
							partJSON, _ = sjson.Set(partJSON, "text", "Continue from: "+prefill)
							syntheticUser, _ = sjson.SetRaw(syntheticUser, "parts.-1", partJSON)
							rawJSON, _ = sjson.SetRawBytes(rawJSON, "request.contents.-1", []byte(syntheticUser))
						}
					}
				}
			}
		}
	}

	toolsResult := gjson.GetBytes(rawJSON, "request.tools")
	if toolsResult.Exists() && toolsResult.IsArray() {
		toolResults := toolsResult.Array()
		for i := 0; i < len(toolResults); i++ {
			functionDeclarationsResult := gjson.GetBytes(rawJSON, fmt.Sprintf("request.tools.%d.function_declarations", i))
			if functionDeclarationsResult.Exists() && functionDeclarationsResult.IsArray() {
				functionDeclarationsResults := functionDeclarationsResult.Array()
				for j := 0; j < len(functionDeclarationsResults); j++ {
					parametersResult := gjson.GetBytes(rawJSON, fmt.Sprintf("request.tools.%d.function_declarations.%d.parameters", i, j))
					if parametersResult.Exists() {
						strJson, _ := util.RenameKey(string(rawJSON), fmt.Sprintf("request.tools.%d.function_declarations.%d.parameters", i, j), fmt.Sprintf("request.tools.%d.function_declarations.%d.parametersJsonSchema", i, j))
						rawJSON = []byte(strJson)
					}
				}
			}
		}
	}

	// Inject tool IDs for functionCall/functionResponse parts that are missing them.
	// Antigravity's Claude backend requires tool_use.id but Gemini clients (e.g. @ai-sdk/google) don't send it.
	type callKey struct {
		name  string
		index int
	}
	callCount := make(map[string]int)
	toolIDMap := make(map[callKey]string)
	respCount := make(map[string]int)

	gjson.GetBytes(rawJSON, "request.contents").ForEach(func(contentIdx, content gjson.Result) bool {
		role := content.Get("role").String()
		if role == "model" {
			content.Get("parts").ForEach(func(partIdx, part gjson.Result) bool {
				if part.Get("functionCall").Exists() {
					if !part.Get("functionCall.id").Exists() || part.Get("functionCall.id").String() == "" {
						funcName := part.Get("functionCall.name").String()
						toolID := generateToolID()
						n := callCount[funcName]
						callCount[funcName] = n + 1
						toolIDMap[callKey{funcName, n}] = toolID
						rawJSON, _ = sjson.SetBytes(rawJSON, fmt.Sprintf("request.contents.%d.parts.%d.functionCall.id", contentIdx.Int(), partIdx.Int()), toolID)
					} else {
						funcName := part.Get("functionCall.name").String()
						n := callCount[funcName]
						callCount[funcName] = n + 1
						toolIDMap[callKey{funcName, n}] = part.Get("functionCall.id").String()
					}
				}
				return true
			})
		} else if role == "user" || role == "function" {
			content.Get("parts").ForEach(func(partIdx, part gjson.Result) bool {
				if part.Get("functionResponse").Exists() {
					if !part.Get("functionResponse.id").Exists() || part.Get("functionResponse.id").String() == "" {
						funcName := part.Get("functionResponse.name").String()
						n := respCount[funcName]
						respCount[funcName] = n + 1
						if id, ok := toolIDMap[callKey{funcName, n}]; ok {
							rawJSON, _ = sjson.SetBytes(rawJSON, fmt.Sprintf("request.contents.%d.parts.%d.functionResponse.id", contentIdx.Int(), partIdx.Int()), id)
						}
					}
				}
				return true
			})
		}
		return true
	})

	// Gemini-specific handling for non-Claude models:
	// - Add skip_thought_signature_validator to functionCall parts so upstream can bypass signature validation.
	// - Also mark thinking parts with the same sentinel when present (we keep the parts; we only annotate them).
	if !strings.Contains(modelName, "claude") {
		const skipSentinel = "skip_thought_signature_validator"

		gjson.GetBytes(rawJSON, "request.contents").ForEach(func(contentIdx, content gjson.Result) bool {
			if content.Get("role").String() == "model" {
				// First pass: collect indices of thinking parts to mark with skip sentinel
				var thinkingIndicesToSkipSignature []int64
				content.Get("parts").ForEach(func(partIdx, part gjson.Result) bool {
					// Collect indices of thinking blocks to mark with skip sentinel
					if part.Get("thought").Bool() {
						thinkingIndicesToSkipSignature = append(thinkingIndicesToSkipSignature, partIdx.Int())
					}
					// Add skip sentinel to functionCall parts
					if part.Get("functionCall").Exists() {
						existingSig := part.Get("thoughtSignature").String()
						if existingSig == "" || len(existingSig) < 50 {
							rawJSON, _ = sjson.SetBytes(rawJSON, fmt.Sprintf("request.contents.%d.parts.%d.thoughtSignature", contentIdx.Int(), partIdx.Int()), skipSentinel)
						}
					}
					return true
				})

				// Add skip_thought_signature_validator sentinel to thinking blocks in reverse order to preserve indices
				for i := len(thinkingIndicesToSkipSignature) - 1; i >= 0; i-- {
					idx := thinkingIndicesToSkipSignature[i]
					rawJSON, _ = sjson.SetBytes(rawJSON, fmt.Sprintf("request.contents.%d.parts.%d.thoughtSignature", contentIdx.Int(), idx), skipSentinel)
				}
			}
			return true
		})
	}

	return common.AttachDefaultSafetySettings(rawJSON, "request.safetySettings")
}

// FunctionCallGroup represents a group of function calls and their responses
type FunctionCallGroup struct {
	ResponsesNeeded int
}

// parseFunctionResponseRaw attempts to normalize a function response part into a JSON object string.
// Falls back to a minimal "functionResponse" object when parsing fails.
func parseFunctionResponseRaw(response gjson.Result) string {
	if response.IsObject() && gjson.Valid(response.Raw) {
		return response.Raw
	}

	log.Debugf("parse function response failed, using fallback")
	funcResp := response.Get("functionResponse")
	if funcResp.Exists() {
		fr := `{"functionResponse":{"name":"","response":{"result":""}}}`
		fr, _ = sjson.Set(fr, "functionResponse.name", funcResp.Get("name").String())
		fr, _ = sjson.Set(fr, "functionResponse.response.result", funcResp.Get("response").String())
		if id := funcResp.Get("id").String(); id != "" {
			fr, _ = sjson.Set(fr, "functionResponse.id", id)
		}
		return fr
	}

	fr := `{"functionResponse":{"name":"unknown","response":{"result":""}}}`
	fr, _ = sjson.Set(fr, "functionResponse.response.result", response.String())
	return fr
}

// fixCLIToolResponse performs sophisticated tool response format conversion and grouping.
// This function transforms the CLI tool response format by intelligently grouping function calls
// with their corresponding responses, ensuring proper conversation flow and API compatibility.
// It converts from a linear format (1.json) to a grouped format (2.json) where function calls
// and their responses are properly associated and structured.
//
// Parameters:
//   - input: The input JSON string to be processed
//
// Returns:
//   - string: The processed JSON string with grouped function calls and responses
//   - error: An error if the processing fails
func fixCLIToolResponse(input string) (string, error) {
	// Parse the input JSON to extract the conversation structure
	parsed := gjson.Parse(input)

	// Extract the contents array which contains the conversation messages
	contents := parsed.Get("request.contents")
	if !contents.Exists() {
		// log.Debugf(input)
		return input, fmt.Errorf("contents not found in input")
	}

	// Initialize data structures for processing and grouping
	contentsWrapper := `{"contents":[]}`
	var pendingGroups []*FunctionCallGroup // Groups awaiting completion with responses
	var collectedResponses []gjson.Result  // Standalone responses to be matched

	// Process each content object in the conversation
	// This iterates through messages and groups function calls with their responses
	contents.ForEach(func(key, value gjson.Result) bool {
		role := value.Get("role").String()
		parts := value.Get("parts")

		// Check if this content has function responses
		var responsePartsInThisContent []gjson.Result
		parts.ForEach(func(_, part gjson.Result) bool {
			if part.Get("functionResponse").Exists() {
				responsePartsInThisContent = append(responsePartsInThisContent, part)
			}
			return true
		})

		// If this content has function responses, collect them
		if len(responsePartsInThisContent) > 0 {
			collectedResponses = append(collectedResponses, responsePartsInThisContent...)

			// Check if any pending groups can be satisfied
			for i := len(pendingGroups) - 1; i >= 0; i-- {
				group := pendingGroups[i]
				if len(collectedResponses) >= group.ResponsesNeeded {
					// Take the needed responses for this group
					groupResponses := collectedResponses[:group.ResponsesNeeded]
					collectedResponses = collectedResponses[group.ResponsesNeeded:]

					// Create merged function response content
					functionResponseContent := `{"parts":[],"role":"function"}`
					for _, response := range groupResponses {
						partRaw := parseFunctionResponseRaw(response)
						if partRaw != "" {
							functionResponseContent, _ = sjson.SetRaw(functionResponseContent, "parts.-1", partRaw)
						}
					}

					if gjson.Get(functionResponseContent, "parts.#").Int() > 0 {
						contentsWrapper, _ = sjson.SetRaw(contentsWrapper, "contents.-1", functionResponseContent)
					}

					// Remove this group as it's been satisfied
					pendingGroups = append(pendingGroups[:i], pendingGroups[i+1:]...)
					break
				}
			}

			return true // Skip adding this content, responses are merged
		}

		// If this is a model with function calls, create a new group
		if role == "model" {
			functionCallsCount := 0
			parts.ForEach(func(_, part gjson.Result) bool {
				if part.Get("functionCall").Exists() {
					functionCallsCount++
				}
				return true
			})

			if functionCallsCount > 0 {
				// Add the model content
				if !value.IsObject() {
					log.Warnf("failed to parse model content")
					return true
				}
				contentsWrapper, _ = sjson.SetRaw(contentsWrapper, "contents.-1", value.Raw)

				// Create a new group for tracking responses
				group := &FunctionCallGroup{
					ResponsesNeeded: functionCallsCount,
				}
				pendingGroups = append(pendingGroups, group)
			} else {
				// Regular model content without function calls
				if !value.IsObject() {
					log.Warnf("failed to parse content")
					return true
				}
				contentsWrapper, _ = sjson.SetRaw(contentsWrapper, "contents.-1", value.Raw)
			}
		} else {
			// Non-model content (user, etc.)
			if !value.IsObject() {
				log.Warnf("failed to parse content")
				return true
			}
			contentsWrapper, _ = sjson.SetRaw(contentsWrapper, "contents.-1", value.Raw)
		}

		return true
	})

	// Handle any remaining pending groups with remaining responses
	for _, group := range pendingGroups {
		if len(collectedResponses) >= group.ResponsesNeeded {
			groupResponses := collectedResponses[:group.ResponsesNeeded]
			collectedResponses = collectedResponses[group.ResponsesNeeded:]

			functionResponseContent := `{"parts":[],"role":"function"}`
			for _, response := range groupResponses {
				partRaw := parseFunctionResponseRaw(response)
				if partRaw != "" {
					functionResponseContent, _ = sjson.SetRaw(functionResponseContent, "parts.-1", partRaw)
				}
			}

			if gjson.Get(functionResponseContent, "parts.#").Int() > 0 {
				contentsWrapper, _ = sjson.SetRaw(contentsWrapper, "contents.-1", functionResponseContent)
			}
		}
	}

	// Update the original JSON with the new contents
	result := input
	result, _ = sjson.SetRaw(result, "request.contents", gjson.Get(contentsWrapper, "contents").Raw)

	return result, nil
}
