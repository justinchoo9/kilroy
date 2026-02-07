package llm

import (
	"context"
	"errors"
)

func wrapContextError(provider string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return NewAbortError(err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewRequestTimeoutError(provider, err.Error())
	}
	return err
}
