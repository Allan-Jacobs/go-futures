package futures

import "strings"

// An aggregate error type
type ErrorAggregation interface {
	error
	Errors() []error
}

type errorAggregation []error

func (e errorAggregation) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	msgs := make([]string, len(e))
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return "[" + strings.Join(msgs, ", ") + "]"
}

func (e errorAggregation) Errors() []error {
	return e
}
