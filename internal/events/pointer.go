package events

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SetPointer replaces an existing value addressed by an RFC 6901 JSON Pointer.
// Simulator overrides are intentionally restricted to descendants of /payload.
func SetPointer(document map[string]any, pointer string, value any) error {
	parent, token, err := pointerParent(document, pointer)
	if err != nil {
		return err
	}
	switch typed := parent.(type) {
	case map[string]any:
		if _, exists := typed[token]; !exists {
			return fmt.Errorf("JSON Pointer target %q does not exist", pointer)
		}
		typed[token] = value
	case []any:
		index, err := arrayIndex(token, len(typed))
		if err != nil {
			return fmt.Errorf("JSON Pointer %q: %w", pointer, err)
		}
		typed[index] = value
	default:
		return fmt.Errorf("JSON Pointer parent for %q is not a container", pointer)
	}
	return nil
}

// UnsetPointer removes an existing object field addressed by an RFC 6901 JSON
// Pointer. Removing array elements is rejected to avoid implicit reindexing.
func UnsetPointer(document map[string]any, pointer string) error {
	parent, token, err := pointerParent(document, pointer)
	if err != nil {
		return err
	}
	switch typed := parent.(type) {
	case map[string]any:
		if _, exists := typed[token]; !exists {
			return fmt.Errorf("JSON Pointer target %q does not exist", pointer)
		}
		delete(typed, token)
	case []any:
		if _, err := arrayIndex(token, len(typed)); err != nil {
			return fmt.Errorf("JSON Pointer %q: %w", pointer, err)
		}
		return errors.New("unsetting array elements is not supported")
	default:
		return fmt.Errorf("JSON Pointer parent for %q is not a container", pointer)
	}
	return nil
}

func pointerParent(document map[string]any, pointer string) (any, string, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, "", err
	}
	if len(tokens) < 2 || tokens[0] != "payload" {
		return nil, "", errors.New("override pointers must start with /payload/")
	}

	var current any = document
	for _, token := range tokens[:len(tokens)-1] {
		switch typed := current.(type) {
		case map[string]any:
			next, exists := typed[token]
			if !exists {
				return nil, "", fmt.Errorf("JSON Pointer parent %q does not exist", pointer)
			}
			current = next
		case []any:
			index, err := arrayIndex(token, len(typed))
			if err != nil {
				return nil, "", fmt.Errorf("JSON Pointer %q: %w", pointer, err)
			}
			current = typed[index]
		default:
			return nil, "", fmt.Errorf("JSON Pointer parent for %q is not a container", pointer)
		}
	}
	return current, tokens[len(tokens)-1], nil
}

func pointerTokens(pointer string) ([]string, error) {
	if pointer == "" || pointer[0] != '/' {
		return nil, errors.New("JSON Pointer must begin with /")
	}
	parts := strings.Split(pointer[1:], "/")
	for index, part := range parts {
		var decoded strings.Builder
		for position := 0; position < len(part); position++ {
			if part[position] != '~' {
				decoded.WriteByte(part[position])
				continue
			}
			if position+1 >= len(part) || (part[position+1] != '0' && part[position+1] != '1') {
				return nil, fmt.Errorf("invalid JSON Pointer escape in %q", pointer)
			}
			if part[position+1] == '0' {
				decoded.WriteByte('~')
			} else {
				decoded.WriteByte('/')
			}
			position++
		}
		parts[index] = decoded.String()
	}
	return parts, nil
}

func arrayIndex(token string, length int) (int, error) {
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 || index >= length {
		return 0, fmt.Errorf("array index %q is out of range", token)
	}
	return index, nil
}
