package zhiyi

import (
	"fmt"
	"strings"
)

// NormalizeWorkitemComments walks a list/search comments payload and normalizes
// each comment map in place (and returns the same root). Safe for nil.
func NormalizeWorkitemComments(out any) any {
	switch v := out.(type) {
	case []any:
		for i, it := range v {
			if m, ok := it.(map[string]any); ok {
				v[i] = NormalizeWorkitemComment(m)
			}
		}
		return v
	case map[string]any:
		for _, key := range []string{"data", "items", "list", "commentList", "result"} {
			if inner, ok := v[key]; ok {
				v[key] = NormalizeWorkitemComments(inner)
			}
		}
		// Single comment object (update RPC may return {comment:{...}} or the comment itself).
		if _, hasContent := v["content"]; hasContent {
			if _, hasUser := v["user"]; hasUser || v["creator"] != nil || v["id"] != nil {
				return NormalizeWorkitemComment(v)
			}
		}
		if c, ok := v["comment"].(map[string]any); ok {
			v["comment"] = NormalizeWorkitemComment(c)
		}
		return v
	default:
		return out
	}
}

// NormalizeWorkitemComment unifies author fields across OAPI list and AccessKey
// update surfaces (#360):
//   - list: user.{id,name} + gmtModified
//   - update: user.displayName + modifiedTime
// Adds displayName/name on user, mirrors a filled creator, and aliases
// modifiedTime from gmtModified when absent.
func NormalizeWorkitemComment(c map[string]any) map[string]any {
	if c == nil {
		return nil
	}
	user := asStringMapAny(c["user"])
	creator := asStringMapAny(c["creator"])
	src := user
	if src == nil {
		src = creator
	}
	if src == nil {
		src = map[string]any{}
	}
	norm := copyStringMap(src)
	id := firstStringField(norm, "id", "identifier")
	name := firstStringField(norm, "name", "displayName", "realName", "nickName")
	display := firstStringField(norm, "displayName", "name", "realName", "nickName")
	if id != "" {
		norm["id"] = id
	}
	if name != "" {
		norm["name"] = name
	}
	if display != "" {
		norm["displayName"] = display
	}
	c["user"] = norm
	// Consumers that look for workitem-style creator.name often miss list's user.name.
	if creator == nil || firstStringField(creator, "name", "displayName") == "" {
		c["creator"] = copyStringMap(norm)
	}
	if _, has := c["modifiedTime"]; !has {
		if gm, ok := c["gmtModified"]; ok && gm != nil {
			c["modifiedTime"] = gm
		}
	}
	if _, has := c["gmtModified"]; !has {
		if mt, ok := c["modifiedTime"]; ok && mt != nil {
			c["gmtModified"] = mt
		}
	}
	return c
}

func asStringMapAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func copyStringMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func firstStringField(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
