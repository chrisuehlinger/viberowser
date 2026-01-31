package dom

import "fmt"

// DOMError represents a DOM exception with a name and message.
type DOMError struct {
	Name    string
	Message string
}

func (e *DOMError) Error() string {
	return fmt.Sprintf("%s: %s", e.Name, e.Message)
}

// Common DOM error constructors

// ErrHierarchyRequest creates a HierarchyRequestError.
func ErrHierarchyRequest(message string) *DOMError {
	return &DOMError{Name: "HierarchyRequestError", Message: message}
}

// ErrNotFound creates a NotFoundError.
func ErrNotFound(message string) *DOMError {
	return &DOMError{Name: "NotFoundError", Message: message}
}

// ErrInvalidCharacter creates an InvalidCharacterError.
func ErrInvalidCharacter(message string) *DOMError {
	return &DOMError{Name: "InvalidCharacterError", Message: message}
}

// ErrNotSupported creates a NotSupportedError.
func ErrNotSupported(message string) *DOMError {
	return &DOMError{Name: "NotSupportedError", Message: message}
}

// ErrInvalidState creates an InvalidStateError.
func ErrInvalidState(message string) *DOMError {
	return &DOMError{Name: "InvalidStateError", Message: message}
}

// ErrIndexSize creates an IndexSizeError.
func ErrIndexSize(message string) *DOMError {
	return &DOMError{Name: "IndexSizeError", Message: message}
}

// ErrWrongDocument creates a WrongDocumentError.
func ErrWrongDocument(message string) *DOMError {
	return &DOMError{Name: "WrongDocumentError", Message: message}
}

// ErrNamespace creates a NamespaceError.
func ErrNamespace(message string) *DOMError {
	return &DOMError{Name: "NamespaceError", Message: message}
}

// ErrInUseAttribute creates an InUseAttributeError.
func ErrInUseAttribute(message string) *DOMError {
	return &DOMError{Name: "InUseAttributeError", Message: message}
}

// ErrSyntax creates a SyntaxError.
func ErrSyntax(message string) *DOMError {
	return &DOMError{Name: "SyntaxError", Message: message}
}
