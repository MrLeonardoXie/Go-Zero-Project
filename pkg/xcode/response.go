package xcode

import (
	"net/http"

	"github.com/MrLeonardoXie/Go-Zero-Project/pkg/xcode/types"
)

func ErrHandler(err error) (int, any) {
	code := CodeFromError(err)

	return http.StatusOK, types.Status{
		Code:    int32(code.Code()),
		Message: code.Message(),
	}
}
