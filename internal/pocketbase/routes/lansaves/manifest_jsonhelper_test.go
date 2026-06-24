package lansaves

import (
	"encoding/json"
	"strings"
)

func jsonMarshalIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
func containsStr(s, sub string) bool          { return strings.Contains(s, sub) }
