// Package gemini provides in-provider request normalization for Gemini API.
// It ensures incoming v1beta requests meet minimal schema requirements
// expected by Google's Generative Language API.
package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/common"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func generateToolID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "toolu_" + hex.EncodeToString(b)
}

// ConvertGeminiRequestToGemini normalizes Gemini v1beta requests.
//   - Adds a default role for each content if missing or invalid.
//     The first message defaults to "user", then alternates user/model when needed.
//
// It keeps the payload otherwise unchanged.
func ConvertGeminiRequestToGemini(_ string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := inputRawJSON
	// Fast path: if no contents field, only attach safety settings
	contents := gjson.GetBytes(rawJSON, "contents")
	if !contents.Exists() {
		return common.AttachDefaultSafetySettings(rawJSON, "safetySettings")
	}

	toolsResult := gjson.GetBytes(rawJSON, "tools")
	if toolsResult.Exists() && toolsResult.IsArray() {
		toolResults := toolsResult.Array()
		for i := 0; i < len(toolResults); i++ {
			if gjson.GetBytes(rawJSON, fmt.Sprintf("tools.%d.functionDeclarations", i)).Exists() {
				strJson, _ := util.RenameKey(string(rawJSON), fmt.Sprintf("tools.%d.functionDeclarations", i), fmt.Sprintf("tools.%d.function_declarations", i))
				rawJSON = []byte(strJson)
			}

			functionDeclarationsResult := gjson.GetBytes(rawJSON, fmt.Sprintf("tools.%d.function_declarations", i))
			if functionDeclarationsResult.Exists() && functionDeclarationsResult.IsArray() {
				functionDeclarationsResults := functionDeclarationsResult.Array()
				for j := 0; j < len(functionDeclarationsResults); j++ {
					parametersResult := gjson.GetBytes(rawJSON, fmt.Sprintf("tools.%d.function_declarations.%d.parameters", i, j))
					if parametersResult.Exists() {
						strJson, _ := util.RenameKey(string(rawJSON), fmt.Sprintf("tools.%d.function_declarations.%d.parameters", i, j), fmt.Sprintf("tools.%d.function_declarations.%d.parametersJsonSchema", i, j))
						rawJSON = []byte(strJson)
					}
				}
			}
		}
	}

	// Walk contents and fix roles
	out := rawJSON
	prevRole := ""
	idx := 0
	contents.ForEach(func(_ gjson.Result, value gjson.Result) bool {
		role := value.Get("role").String()

		// Only user/model are valid for Gemini v1beta requests
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
			path := fmt.Sprintf("contents.%d.role", idx)
			out, _ = sjson.SetBytes(out, path, newRole)
			role = newRole
		}

		prevRole = role
		idx++
		return true
	})

	// Track tool IDs by (functionName, callIndex) so multiple calls to the
	// same function each get their own unique ID that responses can match.
	type callKey struct {
		name  string
		index int
	}
	callCount := make(map[string]int)
	toolIDMap := make(map[callKey]string)
	respCount := make(map[string]int)

	gjson.GetBytes(out, "contents").ForEach(func(key, content gjson.Result) bool {
		contentIdx := key.Int()
		role := content.Get("role").String()

		if role == "model" {
			content.Get("parts").ForEach(func(partKey, part gjson.Result) bool {
				partIdx := partKey.Int()
				if part.Get("functionCall").Exists() {
					out, _ = sjson.SetBytes(out, fmt.Sprintf("contents.%d.parts.%d.thoughtSignature", contentIdx, partIdx), "skip_thought_signature_validator")
					if !part.Get("functionCall.id").Exists() || part.Get("functionCall.id").String() == "" {
						funcName := part.Get("functionCall.name").String()
						toolID := generateToolID()
						n := callCount[funcName]
						callCount[funcName] = n + 1
						toolIDMap[callKey{funcName, n}] = toolID
						out, _ = sjson.SetBytes(out, fmt.Sprintf("contents.%d.parts.%d.functionCall.id", contentIdx, partIdx), toolID)
					}
				} else if part.Get("thoughtSignature").Exists() {
					out, _ = sjson.SetBytes(out, fmt.Sprintf("contents.%d.parts.%d.thoughtSignature", contentIdx, partIdx), "skip_thought_signature_validator")
				}
				return true
			})
		} else if role == "user" {
			content.Get("parts").ForEach(func(partKey, part gjson.Result) bool {
				partIdx := partKey.Int()
				if part.Get("functionResponse").Exists() {
					if !part.Get("functionResponse.id").Exists() || part.Get("functionResponse.id").String() == "" {
						funcName := part.Get("functionResponse.name").String()
						n := respCount[funcName]
						respCount[funcName] = n + 1
						if id, ok := toolIDMap[callKey{funcName, n}]; ok {
							out, _ = sjson.SetBytes(out, fmt.Sprintf("contents.%d.parts.%d.functionResponse.id", contentIdx, partIdx), id)
						}
					}
				}
				return true
			})
		}
		return true
	})

	if gjson.GetBytes(rawJSON, "generationConfig.responseSchema").Exists() {
		strJson, _ := util.RenameKey(string(out), "generationConfig.responseSchema", "generationConfig.responseJsonSchema")
		out = []byte(strJson)
	}

	out = common.AttachDefaultSafetySettings(out, "safetySettings")
	return out
}
