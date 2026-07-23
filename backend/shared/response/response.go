package response

const RequestIDKey = "request_id"

type Response[T any] struct {
	Success   bool   `json:"success"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Data      T      `json:"data,omitempty"`
	Meta      any    `json:"meta,omitempty"`
}

func OK[T any](data T) *Response[T] {
	return &Response[T]{
		Success: true,
		Data:    data,
	}
}

func Created[T any](data T) *Response[T] {
	return &Response[T]{
		Success: true,
		Data:    data,
	}
}

func Error(msg string) *Response[any] {
	return &Response[any]{
		Success: false,
		Message: msg,
	}
}
