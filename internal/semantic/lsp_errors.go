package semantic

import "strings"

// lspMethodError adds a hint when an LSP method is likely unsupported.
func LSPMethodError(method string, err error) string {
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "method not found") {
		return method + " not supported by language server: " + msg
	}
	if strings.Contains(strings.ToLower(msg), "not implemented") {
		return method + " not implemented by language server: " + msg
	}
	return method + " failed: " + msg
}
